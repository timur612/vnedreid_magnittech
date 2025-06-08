import axios from 'axios';
import type { ClusterStats, PodMetrics } from '../types/cluster';
import { mockClusterStats } from '../mocks/cluster';

const API_BASE_URL = 'http://misis.tech:8080/api';

export const getClusterStats = async (): Promise<ClusterStats> => {
    // Временно используем моковые данные
    return mockClusterStats;
    // const { data } = await axios.get(`${API_BASE_URL}/cluster-stats`);
    // return data;
};

export const getPodMetrics = async (namespace: string, podId: string): Promise<PodMetrics> => {
    // Временно используем моковые данные
    const pod = mockClusterStats.pods.find(
        (p) => p.namespace === namespace && p.pod_name === podId
    );
    if (!pod) {
        throw new Error('Pod not found');
    }
    return pod;
    // const { data } = await axios.get(`${API_BASE_URL}/metrics`, {
    //     params: { namespace, 'pod-id': podId },
    // });
    // return data;
};

export const getGrafanaUrl = (podName: string, namespace: string) => {
    return `http://misis.tech:3000/d/6581e46e4e5c7ba40a07646395ef7b23/kubernetes-compute-resources-pod?orgId=1&from=now-1h&to=now&timezone=utc&var-datasource=default&var-cluster=&var-namespace=${namespace}&var-pod=${podName}&refresh=10s`;
};
