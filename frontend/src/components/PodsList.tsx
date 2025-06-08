import { useQuery } from '@tanstack/react-query';
import { Table, Text, Badge, Anchor, Button, Modal } from '@mantine/core';
import { useState } from 'react';
import { getClusterStats, getGrafanaUrl } from '../api/cluster';
import { formatBytes, formatCpu } from '../utils/format';
import { PodDetails } from './PodDetails';
import type { PodMetrics } from '../types/cluster';

export function PodsList() {
    const [selectedPod, setSelectedPod] = useState<PodMetrics | null>(null);
    const { data, isLoading, error } = useQuery({
        queryKey: ['clusterStats'],
        queryFn: getClusterStats,
        refetchInterval: 30000,
    });

    if (isLoading) return <Text>Загрузка...</Text>;
    if (error) return <Text c='red'>Ошибка при загрузке данных</Text>;
    if (!data) return null;

    return (
        <div>
            <Text size='xl' fw={700} mb='md'>
                Общая статистика кластера
            </Text>
            <Table mb='xl'>
                <tbody>
                    <tr>
                        <td>Всего подов:</td>
                        <td>{data.total_pods}</td>
                    </tr>
                    <tr>
                        <td>Текущее использование CPU:</td>
                        <td>{formatCpu(data.total_current_cpu)}</td>
                    </tr>
                    <tr>
                        <td>Текущее использование памяти:</td>
                        <td>{formatBytes(data.total_current_memory)}</td>
                    </tr>
                    <tr>
                        <td>Потенциальная экономия:</td>
                        <td>{formatCpu(data.potential_savings)}</td>
                    </tr>
                </tbody>
            </Table>

            <Text size='xl' fw={700} mb='md'>
                Список подов
            </Text>
            <Table>
                <thead>
                    <tr>
                        <th>Имя пода</th>
                        <th>Namespace</th>
                        <th>CPU</th>
                        <th>Память</th>
                        <th>Score</th>
                        <th>Действия</th>
                    </tr>
                </thead>
                <tbody>
                    {data.pods.map((pod) => (
                        <tr key={pod.pod_name}>
                            <td>{pod.pod_name}</td>
                            <td>{pod.namespace}</td>
                            <td>
                                {formatCpu(pod.current_cpu)} / {formatCpu(pod.max_cpu)}
                            </td>
                            <td>
                                {formatBytes(pod.current_memory)} / {formatBytes(pod.max_memory)}
                            </td>
                            <td>
                                <Badge color={pod.optimization_score > 0.7 ? 'green' : 'red'}>
                                    {pod.optimization_score.toFixed(2)}
                                </Badge>
                            </td>
                            <td>
                                <Button
                                    variant='subtle'
                                    size='xs'
                                    onClick={() => setSelectedPod(pod)}
                                    mr='xs'
                                >
                                    Детали
                                </Button>
                                <Anchor
                                    href={getGrafanaUrl(pod.pod_name, pod.namespace)}
                                    target='_blank'
                                >
                                    Графана
                                </Anchor>
                            </td>
                        </tr>
                    ))}
                </tbody>
            </Table>

            <Modal
                opened={!!selectedPod}
                onClose={() => setSelectedPod(null)}
                title='Детали пода'
                size='xl'
            >
                {selectedPod && (
                    <PodDetails namespace={selectedPod.namespace} podId={selectedPod.pod_name} />
                )}
            </Modal>
        </div>
    );
}
