import type { ClusterStats, PodMetrics, DeadContainer } from '../types/cluster';

const API_URL = 'https://api.misis.tech';

export async function getClusterStats(): Promise<ClusterStats> {
    const response = await fetch(`${API_URL}/api/cluster-stats`);
    if (!response.ok) {
        throw new Error('Failed to fetch cluster stats');
    }
    return response.json();
}

export async function getPodMetrics(namespace: string, podId: string): Promise<PodMetrics> {
    const response = await fetch(`${API_URL}/api/metrics?namespace=${namespace}&pod-id=${podId}`);
    if (!response.ok) {
        if (response.status === 404) {
            throw new Error(`Под ${podId} в namespace ${namespace} не найден`);
        }
        throw new Error(`Ошибка при получении метрик пода: ${response.statusText}`);
    }
    return response.json();
}

export async function updatePodLimits(
    podName: string,
    namespace: string,
    cpu: number,
    memory: number
): Promise<{ message: string; status: string }> {
    const response = await fetch(`${API_URL}/apply-recommendations`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            pod_name: podName,
            namespace,
            cpu,
            memory,
        }),
    });

    if (!response.ok) {
        throw new Error('Ошибка при обновлении лимитов пода');
    }

    return response.json();
}

export function getGrafanaUrl(podName: string, namespace: string): string {
    return `https://grafana.misis.tech/d/6581e46e4e5c7ba40a07646395ef7b23/kubernetes-compute-resources-pod?orgId=1&from=now-1h&to=now&timezone=utc&var-datasource=default&var-cluster=&var-namespace=${namespace}&var-pod=${podName}&refresh=10s`;
}

export async function getDeadContainers(): Promise<DeadContainer[]> {
    const response = await fetch(`${API_URL}/api/dead-containers`);
    if (!response.ok) {
        throw new Error('Ошибка при получении списка мертвых контейнеров');
    }
    return response.json();
}

export async function getLlmRecommendations(podId: string): Promise<{ recommendation: string }> {
    const response = await fetch(`${API_URL}/api/llm-recommendations?pod-id=${podId}`);
    if (!response.ok) {
        throw new Error('Ошибка при получении рекомендаций');
    }
    return response.json();
}
