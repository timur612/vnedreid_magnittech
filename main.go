package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
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
	if len(pod.Spec.Containers) > 0 {
		if cpu := pod.Spec.Containers[0].Resources.Limits.Cpu(); cpu != nil {
			currentCPU = float64(cpu.MilliValue()) / 1000.0
		}
		if memory := pod.Spec.Containers[0].Resources.Limits.Memory(); memory != nil {
			currentMemory = float64(memory.Value())
		}
	}

	cpuQuery := `max(rate(container_cpu_usage_seconds_total{pod="` + podName + `",namespace="` + namespace + `"}[5m]) * 100)`
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
	recommendCPU := maxCPU / 100.0 // Конвертируем проценты в ядра
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
	cpuDelta := stats.TotalCurrentCPU - stats.TotalRecommendCPU
	memDeltaMB := (stats.TotalCurrentMemory - stats.TotalRecommendMem) / (1024 * 1024)
	stats.PotentialSavings = (cpuDelta * ma.config.CPUCostPerCore) + (memDeltaMB * ma.config.MemoryCostPerMB)

	log.Printf("Cluster stats calculated: %d pods, potential savings: %.2f rub", stats.TotalPods, stats.PotentialSavings)
	return stats, nil
}

func (ma *MetricsAnalyzer) formatRecommendation(metrics PodMetrics) string {
	currentMemMB := metrics.CurrentMemory / (1024 * 1024)
	maxMemMB := metrics.MaxMemory / (1024 * 1024)
	recommendMemMB := metrics.RecommendMem / (1024 * 1024)

	cpuDelta := metrics.RecommendCPU - metrics.CurrentCPU
	memDeltaMB := recommendMemMB - currentMemMB
	costDelta := (cpuDelta * ma.config.CPUCostPerCore) + (memDeltaMB * ma.config.MemoryCostPerMB)

	var result string
	result += fmt.Sprintf("Анализ пода: %s\n", metrics.PodName)
	result += fmt.Sprintf("Namespace: %s\n\n", metrics.Namespace)

	result += "Текущие ресурсы:\n"
	result += fmt.Sprintf("CPU: %.2f ядер\n", metrics.CurrentCPU)
	result += fmt.Sprintf("Память: %.2f МБ\n\n", currentMemMB)

	result += "Максимальное использование:\n"
	result += fmt.Sprintf("CPU: %.2f%%\n", metrics.MaxCPU)
	result += fmt.Sprintf("Память: %.2f МБ\n\n", maxMemMB)

	result += "Рекомендации:\n"
	result += fmt.Sprintf("CPU: %.2f ядер (Δ%.2f)\n", metrics.RecommendCPU, cpuDelta)
	result += fmt.Sprintf("Память: %.2f МБ (Δ%.2f)\n", recommendMemMB, memDeltaMB)

	if costDelta < 0 {
		result += fmt.Sprintf("\nЭкономия: %.2f руб.\n", -costDelta)
	} else {
		result += fmt.Sprintf("\nДополнительные затраты: %.2f руб.\n", costDelta)
	}

	return result
}

func main() {
	config := Config{
		CPUCostPerCore:  1000.0, // 1000 рублей за ядро
		MemoryCostPerMB: 0.5,    // 0.5 рублей за МБ
		PrometheusURL:   "http://localhost:9090",
		KubeconfigPath:  "/home/ilinivan/.kube/config",
	}

	analyzer, err := NewMetricsAnalyzer(config)
	if err != nil {
		log.Fatalf("Failed to create metrics analyzer: %v", err)
	}

	// JSON API
	http.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		podID := r.URL.Query().Get("pod-id")
		w.Header().Set("Content-Type", "application/json")

		if podID != "" {
			metrics, err := analyzer.getMetricsForPod(podID, namespace)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error getting metrics: %v", err), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(metrics)
			return
		}

		pods, err := analyzer.k8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting pods: %v", err), http.StatusInternalServerError)
			return
		}

		var podMetrics []PodMetrics
		for _, pod := range pods.Items {
			metrics, err := analyzer.getMetricsForPod(pod.Name, namespace)
			if err != nil {
				log.Printf("Error getting metrics for pod %s: %v", pod.Name, err)
				continue
			}
			podMetrics = append(podMetrics, metrics)
		}

		json.NewEncoder(w).Encode(podMetrics)
	})

	// Текстовый API
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		podID := r.URL.Query().Get("pod-id")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if podID != "" {
			metrics, err := analyzer.getMetricsForPod(podID, namespace)
			if err != nil {
				http.Error(w, fmt.Sprintf("Ошибка получения метрик: %v", err), http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, analyzer.formatRecommendation(metrics))
			return
		}

		pods, err := analyzer.k8sClient.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка получения списка подов: %v", err), http.StatusInternalServerError)
			return
		}

		var response string
		for _, pod := range pods.Items {
			metrics, err := analyzer.getMetricsForPod(pod.Name, namespace)
			if err != nil {
				log.Printf("Error getting metrics for pod %s: %v", pod.Name, err)
				continue
			}
			response += analyzer.formatRecommendation(metrics) + "\n---\n\n"
		}

		fmt.Fprint(w, response)
	})

	// API для статистики кластера
	http.HandleFunc("/api/cluster-stats", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request for cluster stats")
		w.Header().Set("Content-Type", "application/json")
		stats, err := analyzer.getClusterStats()
		if err != nil {
			log.Printf("Error getting cluster stats: %v", err)
			http.Error(w, fmt.Sprintf("Error getting cluster stats: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Sending cluster stats response")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			log.Printf("Error encoding cluster stats: %v", err)
			http.Error(w, fmt.Sprintf("Error encoding cluster stats: %v", err), http.StatusInternalServerError)
			return
		}
	})

	log.Printf("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
