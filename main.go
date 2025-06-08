package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	CPUCostPerCore  float64 // Стоимость одного ядра в рублях
	MemoryCostPerMB float64 // Стоимость одного МБ памяти в рублях
	PrometheusURL   string
	KubeconfigPath  string
}

type PodMetrics struct {
	PodName           string  `json:"pod_name"`
	Namespace         string  `json:"namespace"`
	CurrentCPU        float64 `json:"current_cpu"`
	CurrentMemory     float64 `json:"current_memory"`
	MaxCPU            float64 `json:"max_cpu"`
	MaxMemory         float64 `json:"max_memory"`
	RecommendCPU      float64 `json:"recommend_cpu"`
	RecommendMem      float64 `json:"recommend_memory"`
	OptimizationScore float64 `json:"optimization_score"` // Чем выше, тем больше необходимость оптимизации
}

type ClusterStats struct {
	TotalPods          int          `json:"total_pods"`
	TotalCurrentCPU    float64      `json:"total_current_cpu"`
	TotalCurrentMemory float64      `json:"total_current_memory"`
	TotalMaxCPU        float64      `json:"total_max_cpu"`
	TotalMaxMemory     float64      `json:"total_max_memory"`
	TotalRecommendCPU  float64      `json:"total_recommend_cpu"`
	TotalRecommendMem  float64      `json:"total_recommend_memory"`
	PotentialSavings   float64      `json:"potential_savings"`
	Pods               []PodMetrics `json:"pods"`
}

type ResourceRequest struct {
	PodName   string  `json:"pod_name"`
	Namespace string  `json:"namespace"`
	CPU       float64 `json:"cpu"`     // в миллияхдрах (например, 1000m = 1 ядро)
	Memory    float64 `json:"memory"`  // в байтах
	Storage   float64 `json:"storage"` // в байтах
}

type DeadContainer struct {
	PodName       string  `json:"pod_name"`
	Namespace     string  `json:"namespace"`
	LastActivity  string  `json:"last_activity"`
	NetworkIn     float64 `json:"network_in_bytes"`
	NetworkOut    float64 `json:"network_out_bytes"`
	ContainerName string  `json:"container_name"`
	PodType       string  `json:"pod_type"` // Тип пода (Deployment, StatefulSet и т.д.)
}

type LLMRequest struct {
	Cluster string    `json:"cluster"`
	Pod     string    `json:"pod"`
	CPUData []float64 `json:"cpu_data"`
	RAMData []float64 `json:"ram_data"`
	CPUCost float64   `json:"cpu_cost"`
	RAMCost float64   `json:"ram_cost"`
}

type LLMResponse struct {
	Recommendation string `json:"recommendation"`
}

type MetricsAnalyzer struct {
	promClient v1.API
	k8sClient  *kubernetes.Clientset
	config     Config
}

func NewMetricsAnalyzer(config Config) (*MetricsAnalyzer, error) {
	promClient, err := api.NewClient(api.Config{
		Address: config.PrometheusURL,
	})
	if err != nil {
		return nil, err
	}

	k8sConfig, err := clientcmd.BuildConfigFromFlags("", config.KubeconfigPath)
	if err != nil {
		return nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}

	return &MetricsAnalyzer{
		promClient: v1.NewAPI(promClient),
		k8sClient:  k8sClient,
		config:     config,
	}, nil
}

func (ma *MetricsAnalyzer) getMetricsForPod(podName string, namespace string) (PodMetrics, error) {
	pod, err := ma.k8sClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return PodMetrics{}, err
	}

	var currentCPU, currentMemory float64
	// Суммируем лимиты всех контейнеров в поде
	for _, container := range pod.Spec.Containers {
		if cpu := container.Resources.Limits.Cpu(); cpu != nil {
			currentCPU += float64(cpu.MilliValue()) // Теперь храним в миллипроцессорах
		}
		if memory := container.Resources.Limits.Memory(); memory != nil {
			currentMemory += float64(memory.Value())
		}
	}

	cpuQuery := `max(rate(container_cpu_usage_seconds_total{pod="` + podName + `",namespace="` + namespace + `"}[5m]) * 1000)` // Умножаем на 1000 для получения миллипроцессоров
	cpuResult, _, err := ma.promClient.Query(context.Background(), cpuQuery, time.Now())
	if err != nil {
		return PodMetrics{}, err
	}

	memQuery := `max(container_memory_usage_bytes{pod="` + podName + `",namespace="` + namespace + `"})`
	memResult, _, err := ma.promClient.Query(context.Background(), memQuery, time.Now())
	if err != nil {
		return PodMetrics{}, err
	}

	var maxCPU, maxMemory float64
	if cpuResult.Type() == model.ValVector {
		vector := cpuResult.(model.Vector)
		if len(vector) > 0 {
			maxCPU = float64(vector[0].Value)
		}
	}

	if memResult.Type() == model.ValVector {
		vector := memResult.(model.Vector)
		if len(vector) > 0 {
			maxMemory = float64(vector[0].Value)
		}
	}

	// Рекомендации с учетом текущих лимитов
	recommendCPU := maxCPU // Теперь уже в миллипроцессорах
	recommendMem := maxMemory * 1.2

	// Вычисляем score для сортировки (чем больше разница между текущими и рекомендуемыми ресурсами, тем выше score)
	var cpuDiff, memDiff float64

	// Проверяем деление на ноль для CPU
	if currentCPU > 0 {
		cpuDiff = (currentCPU - recommendCPU) / currentCPU
	} else if recommendCPU > 0 {
		cpuDiff = 1.0 // Если текущий CPU = 0, а рекомендуемый > 0, считаем что разница максимальная
	} else {
		cpuDiff = 0.0 // Если оба значения = 0, считаем что разницы нет
	}

	// Проверяем деление на ноль для памяти
	if currentMemory > 0 {
		memDiff = (currentMemory - recommendMem) / currentMemory
	} else if recommendMem > 0 {
		memDiff = 1.0 // Если текущая память = 0, а рекомендуемая > 0, считаем что разница максимальная
	} else {
		memDiff = 0.0 // Если оба значения = 0, считаем что разницы нет
	}

	optimizationScore := (cpuDiff + memDiff) / 2

	return PodMetrics{
		PodName:           podName,
		Namespace:         namespace,
		CurrentCPU:        currentCPU,
		CurrentMemory:     currentMemory,
		MaxCPU:            maxCPU,
		MaxMemory:         maxMemory,
		RecommendCPU:      recommendCPU,
		RecommendMem:      recommendMem,
		OptimizationScore: optimizationScore,
	}, nil
}

func (ma *MetricsAnalyzer) getClusterStats() (ClusterStats, error) {
	log.Printf("Getting cluster stats...")
	namespaces, err := ma.k8sClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error getting namespaces: %v", err)
		return ClusterStats{}, err
	}
	log.Printf("Found %d namespaces", len(namespaces.Items))

	var stats ClusterStats
	var allPods []PodMetrics

	for _, ns := range namespaces.Items {
		log.Printf("Processing namespace: %s", ns.Name)
		pods, err := ma.k8sClient.CoreV1().Pods(ns.Name).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Error getting pods in namespace %s: %v", ns.Name, err)
			continue
		}
		log.Printf("Found %d pods in namespace %s", len(pods.Items), ns.Name)

		for _, pod := range pods.Items {
			log.Printf("Getting metrics for pod %s in namespace %s", pod.Name, ns.Name)
			metrics, err := ma.getMetricsForPod(pod.Name, ns.Name)
			if err != nil {
				log.Printf("Error getting metrics for pod %s: %v", pod.Name, err)
				continue
			}

			stats.TotalCurrentCPU += metrics.CurrentCPU
			stats.TotalCurrentMemory += metrics.CurrentMemory
			stats.TotalMaxCPU += metrics.MaxCPU
			stats.TotalMaxMemory += metrics.MaxMemory
			stats.TotalRecommendCPU += metrics.RecommendCPU
			stats.TotalRecommendMem += metrics.RecommendMem

			allPods = append(allPods, metrics)
		}
	}

	// Сортируем поды по score (по убыванию)
	sort.Slice(allPods, func(i, j int) bool {
		return allPods[i].OptimizationScore > allPods[j].OptimizationScore
	})

	stats.TotalPods = len(allPods)
	stats.Pods = allPods

	// Рассчитываем потенциальную экономию
	cpuDelta := (stats.TotalCurrentCPU - stats.TotalRecommendCPU) / 1000 // Конвертируем в ядра
	memDeltaMB := (stats.TotalCurrentMemory - stats.TotalRecommendMem) / (1024 * 1024)
	stats.PotentialSavings = (cpuDelta * ma.config.CPUCostPerCore) + (memDeltaMB * ma.config.MemoryCostPerMB)

	log.Printf("Cluster stats calculated: %d pods, potential savings: %.2f rub", stats.TotalPods, stats.PotentialSavings)
	return stats, nil
}

func (ma *MetricsAnalyzer) formatRecommendation(metrics PodMetrics) string {
	currentMemMB := metrics.CurrentMemory / (1024 * 1024)
	maxMemMB := metrics.MaxMemory / (1024 * 1024)
	recommendMemMB := metrics.RecommendMem / (1024 * 1024)

	// Конвертируем CPU в ядра для расчёта стоимости
	currentCPUCores := metrics.CurrentCPU / 1000
	recommendCPUCores := metrics.RecommendCPU / 1000
	cpuDelta := recommendCPUCores - currentCPUCores
	memDeltaMB := recommendMemMB - currentMemMB
	costDelta := (cpuDelta * ma.config.CPUCostPerCore) + (memDeltaMB * ma.config.MemoryCostPerMB)

	var result string
	result += fmt.Sprintf("Анализ пода: %s\n", metrics.PodName)
	result += fmt.Sprintf("Namespace: %s\n\n", metrics.Namespace)

	result += "Текущие ресурсы:\n"
	result += fmt.Sprintf("CPU: %.0fm (%.2f ядер)\n", metrics.CurrentCPU, currentCPUCores)
	result += fmt.Sprintf("Память: %.2f МБ\n\n", currentMemMB)

	result += "Максимальное использование:\n"
	result += fmt.Sprintf("CPU: %.0fm (%.2f ядер)\n", metrics.MaxCPU, metrics.MaxCPU/1000)
	result += fmt.Sprintf("Память: %.2f МБ\n\n", maxMemMB)

	result += "Рекомендации:\n"
	result += fmt.Sprintf("CPU: %.0fm (%.2f ядер) (Δ%.2f)\n", metrics.RecommendCPU, recommendCPUCores, cpuDelta)
	result += fmt.Sprintf("Память: %.2f МБ (Δ%.2f)\n", recommendMemMB, memDeltaMB)

	if costDelta < 0 {
		result += fmt.Sprintf("\nЭкономия: %.2f руб.\n", -costDelta)
	} else {
		result += fmt.Sprintf("\nДополнительные затраты: %.2f руб.\n", costDelta)
	}

	return result
}

func (ma *MetricsAnalyzer) applyRecommendations(req ResourceRequest) error {
	// Получаем под для определения его владельца
	pod, err := ma.k8sClient.CoreV1().Pods(req.Namespace).Get(context.Background(), req.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ошибка получения пода: %v", err)
	}

	// Получаем владельца пода (Deployment или StatefulSet)
	var ownerRef *metav1.OwnerReference
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "ReplicaSet" || ref.Kind == "StatefulSet" {
			ownerRef = &ref
			break
		}
	}

	if ownerRef == nil {
		return fmt.Errorf("под не принадлежит Deployment или StatefulSet")
	}

	// Если под принадлежит ReplicaSet, получаем Deployment
	if ownerRef.Kind == "ReplicaSet" {
		rs, err := ma.k8sClient.AppsV1().ReplicaSets(req.Namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ошибка получения ReplicaSet: %v", err)
		}

		// Получаем Deployment
		for _, ref := range rs.OwnerReferences {
			if ref.Kind == "Deployment" {
				deployment, err := ma.k8sClient.AppsV1().Deployments(req.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("ошибка получения Deployment: %v", err)
				}

				// Обновляем ресурсы в Deployment
				for i := range deployment.Spec.Template.Spec.Containers {
					container := &deployment.Spec.Template.Spec.Containers[i]

					if req.CPU > 0 {
						cpuQuantity := resource.NewMilliQuantity(int64(req.CPU), resource.DecimalSI)
						container.Resources.Limits[corev1.ResourceCPU] = *cpuQuantity
						container.Resources.Requests[corev1.ResourceCPU] = *cpuQuantity
					}

					if req.Memory > 0 {
						memQuantity := resource.NewQuantity(int64(req.Memory), resource.BinarySI)
						container.Resources.Limits[corev1.ResourceMemory] = *memQuantity
						container.Resources.Requests[corev1.ResourceMemory] = *memQuantity
					}

					if req.Storage > 0 {
						storageQuantity := resource.NewQuantity(int64(req.Storage), resource.BinarySI)
						container.Resources.Limits[corev1.ResourceEphemeralStorage] = *storageQuantity
						container.Resources.Requests[corev1.ResourceEphemeralStorage] = *storageQuantity
					}
				}

				// Применяем изменения к Deployment
				_, err = ma.k8sClient.AppsV1().Deployments(req.Namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("ошибка обновления Deployment: %v", err)
				}

				return nil
			}
		}
		return fmt.Errorf("не найден Deployment для пода")
	}

	// Если под принадлежит StatefulSet
	if ownerRef.Kind == "StatefulSet" {
		statefulSet, err := ma.k8sClient.AppsV1().StatefulSets(req.Namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ошибка получения StatefulSet: %v", err)
		}

		// Обновляем ресурсы в StatefulSet
		for i := range statefulSet.Spec.Template.Spec.Containers {
			container := &statefulSet.Spec.Template.Spec.Containers[i]

			if req.CPU > 0 {
				cpuQuantity := resource.NewMilliQuantity(int64(req.CPU), resource.DecimalSI)
				container.Resources.Limits[corev1.ResourceCPU] = *cpuQuantity
				container.Resources.Requests[corev1.ResourceCPU] = *cpuQuantity
			}

			if req.Memory > 0 {
				memQuantity := resource.NewQuantity(int64(req.Memory), resource.BinarySI)
				container.Resources.Limits[corev1.ResourceMemory] = *memQuantity
				container.Resources.Requests[corev1.ResourceMemory] = *memQuantity
			}

			if req.Storage > 0 {
				storageQuantity := resource.NewQuantity(int64(req.Storage), resource.BinarySI)
				container.Resources.Limits[corev1.ResourceEphemeralStorage] = *storageQuantity
				container.Resources.Requests[corev1.ResourceEphemeralStorage] = *storageQuantity
			}
		}

		// Применяем изменения к StatefulSet
		_, err = ma.k8sClient.AppsV1().StatefulSets(req.Namespace).Update(context.Background(), statefulSet, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка обновления StatefulSet: %v", err)
		}

		return nil
	}

	return fmt.Errorf("неподдерживаемый тип владельца пода")
}

func (ma *MetricsAnalyzer) findDeadContainers() ([]DeadContainer, error) {
	log.Printf("Searching for dead containers in namespace default...")

	var deadContainers []DeadContainer

	// Получаем поды только из namespace default
	pods, err := ma.k8sClient.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка получения подов в namespace default: %v", err)
	}

	for _, pod := range pods.Items {
		// Пропускаем поды без владельца (обычно это системные поды)
		if len(pod.OwnerReferences) == 0 {
			continue
		}

		for _, container := range pod.Spec.Containers {
			// Проверяем сетевую активность за последние 12 часов
			networkQuery := fmt.Sprintf(
				`max_over_time(rate(container_network_receive_bytes_total{pod="%s",namespace="default",container="%s"}[5m])[12h:])`,
				pod.Name, container.Name,
			)

			networkResult, _, err := ma.promClient.Query(context.Background(), networkQuery, time.Now())
			if err != nil {
				log.Printf("Ошибка получения метрик сети для пода %s: %v", pod.Name, err)
				continue
			}

			// Получаем последнюю активность
			lastActivityQuery := fmt.Sprintf(
				`max(container_network_receive_bytes_total{pod="%s",namespace="default",container="%s"})`,
				pod.Name, container.Name,
			)

			lastActivityResult, _, err := ma.promClient.Query(context.Background(), lastActivityQuery, time.Now())
			if err != nil {
				log.Printf("Ошибка получения времени последней активности для пода %s: %v", pod.Name, err)
				continue
			}

			var networkIn, networkOut float64
			var lastActivity time.Time

			// Обрабатываем результаты запросов
			if networkResult.Type() == model.ValVector {
				vector := networkResult.(model.Vector)
				if len(vector) > 0 {
					networkIn = float64(vector[0].Value)
				}
			}

			if lastActivityResult.Type() == model.ValVector {
				vector := lastActivityResult.(model.Vector)
				if len(vector) > 0 {
					lastActivity = vector[0].Timestamp.Time()
				}
			}

			// Если нет сетевой активности за последние 12 часов
			if networkIn == 0 {
				// Получаем тип пода (Deployment, StatefulSet и т.д.)
				podType := "Unknown"
				if len(pod.OwnerReferences) > 0 {
					owner := pod.OwnerReferences[0]
					if owner.Kind == "ReplicaSet" {
						rs, err := ma.k8sClient.AppsV1().ReplicaSets("default").Get(context.Background(), owner.Name, metav1.GetOptions{})
						if err == nil && len(rs.OwnerReferences) > 0 {
							podType = rs.OwnerReferences[0].Kind
						}
					} else {
						podType = owner.Kind
					}
				}

				deadContainers = append(deadContainers, DeadContainer{
					PodName:       pod.Name,
					Namespace:     "default",
					LastActivity:  lastActivity.Format(time.RFC3339),
					NetworkIn:     networkIn,
					NetworkOut:    networkOut,
					ContainerName: container.Name,
					PodType:       podType,
				})
			}
		}
	}

	return deadContainers, nil
}

func (ma *MetricsAnalyzer) getLLMRecommendations(podName string) (string, error) {
	// Получаем метрики за последние 12 часов
	cpuQuery := fmt.Sprintf(
		`rate(container_cpu_usage_seconds_total{pod="%s",namespace="default"}[5m])[12h:] * 1000`, // Умножаем на 1000 для получения миллипроцессоров
		podName,
	)
	ramQuery := fmt.Sprintf(
		`container_memory_usage_bytes{pod="%s",namespace="default"}[12h:]`,
		podName,
	)

	log.Printf("Executing CPU query: %s", cpuQuery)
	cpuResult, _, err := ma.promClient.Query(context.Background(), cpuQuery, time.Now())
	if err != nil {
		return "", fmt.Errorf("ошибка получения CPU метрик: %v", err)
	}

	log.Printf("Executing RAM query: %s", ramQuery)
	ramResult, _, err := ma.promClient.Query(context.Background(), ramQuery, time.Now())
	if err != nil {
		return "", fmt.Errorf("ошибка получения RAM метрик: %v", err)
	}

	// Извлекаем данные из результатов
	var cpuData, ramData []float64

	if cpuResult.Type() == model.ValMatrix {
		matrix := cpuResult.(model.Matrix)
		for _, stream := range matrix {
			for _, point := range stream.Values {
				cpuData = append(cpuData, float64(point.Value))
			}
		}
	}
	log.Printf("Collected %d CPU data points", len(cpuData))

	if ramResult.Type() == model.ValMatrix {
		matrix := ramResult.(model.Matrix)
		for _, stream := range matrix {
			for _, point := range stream.Values {
				// Конвертируем байты в МБ
				ramData = append(ramData, float64(point.Value)/1024/1024)
			}
		}
	}
	log.Printf("Collected %d RAM data points", len(ramData))

	// Проверяем, что у нас есть данные
	if len(cpuData) == 0 || len(ramData) == 0 {
		return "", fmt.Errorf("недостаточно данных для анализа: CPU points=%d, RAM points=%d", len(cpuData), len(ramData))
	}

	// Формируем запрос к LLM сервису
	llmRequest := LLMRequest{
		Cluster: "default",
		Pod:     podName,
		CPUData: cpuData,
		RAMData: ramData,
		CPUCost: ma.config.CPUCostPerCore,
		RAMCost: ma.config.MemoryCostPerMB,
	}

	// Отправляем запрос
	jsonData, err := json.Marshal(llmRequest)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации запроса: %v", err)
	}

	log.Printf("Sending request to LLM service: %s", string(jsonData))

	resp, err := http.Post("https://useful-kite-settled.ngrok-free.app/get_llm_rec", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка отправки запроса: %v", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа для логирования
	body, _ := io.ReadAll(resp.Body)
	log.Printf("LLM service response status: %d, body: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка от сервера: %d, body: %s", resp.StatusCode, string(body))
	}

	var llmResponse LLMResponse
	if err := json.Unmarshal(body, &llmResponse); err != nil {
		return "", fmt.Errorf("ошибка десериализации ответа: %v", err)
	}

	return llmResponse.Recommendation, nil
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// Logging middleware
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Логируем начало запроса с параметрами
		log.Printf("Started %s %s", r.Method, r.URL.String())
		log.Printf("Query parameters: %v", r.URL.Query())

		// Для POST запросов логируем тело
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Error reading request body: %v", err)
			} else {
				log.Printf("Request body: %s", string(body))
				// Восстанавливаем тело запроса для дальнейшего использования
				r.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		}

		// Создаем ResponseWriter для перехвата статуса ответа
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next(rw, r)

		// Логируем завершение запроса с кодом ответа и временем выполнения
		log.Printf("Completed %s %s with status %d in %v",
			r.Method,
			r.URL.String(),
			rw.statusCode,
			time.Since(start))
	}
}

// responseWriter перехватывает статус ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	config := Config{
		CPUCostPerCore:  1000.0, // Примерная стоимость ядра в рублях
		MemoryCostPerMB: 0.1,    // Примерная стоимость МБ памяти в рублях
		PrometheusURL:   "http://0.0.0.0:9090",
		KubeconfigPath:  "/home/ilinivan/.kube/config",
	}

	analyzer, err := NewMetricsAnalyzer(config)
	if err != nil {
		log.Fatalf("Error creating metrics analyzer: %v", err)
	}

	// Старый эндпоинт для обратной совместимости
	http.HandleFunc("/metrics", corsMiddleware(loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		stats, err := analyzer.getClusterStats()
		if err != nil {
			log.Printf("Error getting cluster stats: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})))

	// Новый эндпоинт с сортировкой
	http.HandleFunc("/api/cluster-stats", corsMiddleware(loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		stats, err := analyzer.getClusterStats()
		if err != nil {
			log.Printf("Error getting cluster stats: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Сортируем поды по убыванию optimization_score
		sort.Slice(stats.Pods, func(i, j int) bool {
			return stats.Pods[i].OptimizationScore > stats.Pods[j].OptimizationScore
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})))

	// Эндпоинт для получения метрик конкретного пода
	http.HandleFunc("/api/metrics", corsMiddleware(loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		namespace := r.URL.Query().Get("namespace")
		podID := r.URL.Query().Get("pod-id")

		log.Printf("Getting metrics for pod %s in namespace %s", podID, namespace)

		if namespace == "" || podID == "" {
			log.Printf("Missing required parameters: namespace=%s, pod-id=%s", namespace, podID)
			http.Error(w, "namespace and pod-id parameters are required", http.StatusBadRequest)
			return
		}

		metrics, err := analyzer.getMetricsForPod(podID, namespace)
		if err != nil {
			log.Printf("Error getting metrics for pod %s: %v", podID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})))

	http.HandleFunc("/apply-recommendations", corsMiddleware(loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Printf("Invalid method %s for /apply-recommendations", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request ResourceRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			log.Printf("Error decoding request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		log.Printf("Applying recommendations for pod %s in namespace %s with CPU=%f, Memory=%f, Storage=%f",
			request.PodName, request.Namespace, request.CPU, request.Memory, request.Storage)

		if request.PodName == "" || request.Namespace == "" {
			log.Printf("Missing required fields: pod_name=%s, namespace=%s", request.PodName, request.Namespace)
			http.Error(w, "pod_name and namespace are required", http.StatusBadRequest)
			return
		}

		err := analyzer.applyRecommendations(request)
		if err != nil {
			log.Printf("Error applying recommendations for pod %s: %v", request.PodName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": fmt.Sprintf("Ресурсы успешно обновлены для пода %s в namespace %s", request.PodName, request.Namespace),
		})
	})))

	// Эндпоинт для поиска мертвых контейнеров
	http.HandleFunc("/api/dead-containers", corsMiddleware(loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		deadContainers, err := analyzer.findDeadContainers()
		if err != nil {
			log.Printf("Error finding dead containers: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deadContainers)
	})))

	// Эндпоинт для получения рекомендаций от LLM
	http.HandleFunc("/api/llm-recommendations", corsMiddleware(loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		podID := r.URL.Query().Get("pod-id")
		if podID == "" {
			http.Error(w, "pod-id parameter is required", http.StatusBadRequest)
			return
		}

		recommendation, err := analyzer.getLLMRecommendations(podID)
		if err != nil {
			log.Printf("Error getting LLM recommendations for pod %s: %v", podID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"recommendation": recommendation,
		})
	})))

	log.Printf("Server starting on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
