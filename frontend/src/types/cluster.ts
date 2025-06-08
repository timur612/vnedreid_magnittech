export interface PodMetrics {
    pod_name: string;
    namespace: string;
    current_cpu: number;
    current_memory: number;
    max_cpu: number;
    max_memory: number;
    recommend_cpu: number;
    recommend_memory: number;
    optimization_score: number;
}

export interface ClusterStats {
    total_pods: number;
    total_current_cpu: number;
    total_current_memory: number;
    total_max_cpu: number;
    total_max_memory: number;
    total_recommend_cpu: number;
    total_recommend_memory: number;
    potential_savings: number;
    pods: PodMetrics[];
}

export interface DeadContainer {
    pod_name: string;
    namespace: string;
    last_activity: string;
    network_in_bytes: number;
    network_out_bytes: number;
    container_name: string;
    pod_type: string;
}
