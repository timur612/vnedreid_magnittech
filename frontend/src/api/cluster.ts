import type { ClusterStats, PodMetrics } from '../types/cluster';

const API_URL = 'http://misis.tech:8080';

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

export function getGrafanaUrl(podName: string, namespace: string): string {
    return `http://misis.tech:3000/d/6581e46e4e5c7ba40a07646395ef7b23/kubernetes-compute-resources-pod?orgId=1&from=now-1h&to=now&timezone=utc&var-datasource=default&var-cluster=&var-namespace=${namespace}&var-pod=${podName}&refresh=10s`;
}
