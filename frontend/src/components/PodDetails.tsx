import { useQuery } from '@tanstack/react-query';
import { Card, Text, Group, Badge, Stack, Anchor } from '@mantine/core';
import { getPodMetrics, getGrafanaUrl } from '../api/cluster';
import { formatBytes, formatCpu } from '../utils/format';
import type { PodMetrics } from '../types/cluster';

interface PodDetailsProps {
    namespace: string;
    podId: string;
}

export function PodDetails({ namespace, podId }: PodDetailsProps) {
    const { data, isLoading, error } = useQuery<PodMetrics>({
        queryKey: ['podMetrics', namespace, podId],
        queryFn: () => getPodMetrics(namespace, podId),
        refetchInterval: 30000,
    });

    if (isLoading) return <Text>Загрузка...</Text>;
    if (error) return <Text c='red'>Ошибка при загрузке данных</Text>;
    if (!data) return null;

    return (
        <Card shadow='sm' p='lg' radius='md' withBorder>
            <Stack gap='md'>
                <Group justify='space-between'>
                    <Text size='xl' fw={700}>
                        {data.pod_name}
                    </Text>
                    <Badge size='lg' color={data.optimization_score > 0.7 ? 'green' : 'red'}>
                        Score: {data.optimization_score.toFixed(2)}
                    </Badge>
                </Group>

                <Text>Namespace: {data.namespace}</Text>

                <Group grow>
                    <Card withBorder p='md'>
                        <Text fw={500} mb='xs'>
                            CPU
                        </Text>
                        <Text>Текущее: {formatCpu(data.current_cpu)}</Text>
                        <Text>Максимальное: {formatCpu(data.max_cpu)}</Text>
                        <Text>Рекомендуемое: {formatCpu(data.recommend_cpu)}</Text>
                    </Card>

                    <Card withBorder p='md'>
                        <Text fw={500} mb='xs'>
                            Память
                        </Text>
                        <Text>Текущая: {formatBytes(data.current_memory)}</Text>
                        <Text>Максимальная: {formatBytes(data.max_memory)}</Text>
                        <Text>Рекомендуемая: {formatBytes(data.recommend_memory)}</Text>
                    </Card>
                </Group>

                <Anchor href={getGrafanaUrl(data.pod_name, data.namespace)} target='_blank'>
                    Открыть в Grafana
                </Anchor>
            </Stack>
        </Card>
    );
}
