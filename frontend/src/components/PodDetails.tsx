import { useQuery } from '@tanstack/react-query';
import { Text, Group, Badge, Stack, Anchor, Progress, Paper, Title, Grid } from '@mantine/core';
import { getPodMetrics, getGrafanaUrl } from '../api/cluster';
import { formatBytes, formatCpu } from '../utils/format';
import type { PodMetrics } from '../types/cluster';
import { IconChartBar } from '@tabler/icons-react';

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

    const cpuUsagePercent = (data.current_cpu / data.max_cpu) * 100;
    const memoryUsagePercent = (data.current_memory / data.max_memory) * 100;

    return (
        <Stack gap='xl'>
            <Group justify='space-between'>
                <Stack gap='xs'>
                    <Text size='sm' c='dimmed'>
                        Имя пода
                    </Text>
                    <Title order={3} c='blue.7'>
                        {data.pod_name}
                    </Title>
                </Stack>
                <Group>
                    <Badge size='lg' color={data.optimization_score > 0.7 ? 'green' : 'red'}>
                        Score: {data.optimization_score.toFixed(2)}
                    </Badge>
                    <Badge variant='light' color='blue'>
                        {data.namespace}
                    </Badge>
                </Group>
            </Group>

            <Grid>
                <Grid.Col span={6}>
                    <Paper shadow='sm' p='md' radius='md' withBorder>
                        <Stack gap='md'>
                            <Title order={4} c='blue.7'>
                                CPU
                            </Title>
                            <Stack gap='xs'>
                                <Group justify='space-between'>
                                    <Text size='sm' c='dimmed'>
                                        Текущее использование
                                    </Text>
                                    <Text fw={500} c='blue.7'>
                                        {formatCpu(data.current_cpu)}
                                    </Text>
                                </Group>
                                <Progress
                                    value={cpuUsagePercent}
                                    color={data.current_cpu > data.recommend_cpu ? 'red' : 'green'}
                                    size='md'
                                />
                            </Stack>
                            <Stack gap='xs'>
                                <Group justify='space-between'>
                                    <Text size='sm' c='dimmed'>
                                        Максимальное использование
                                    </Text>
                                    <Text fw={500} c='blue.7'>
                                        {formatCpu(data.max_cpu)}
                                    </Text>
                                </Group>
                            </Stack>
                            <Stack gap='xs'>
                                <Group justify='space-between'>
                                    <Text size='sm' c='dimmed'>
                                        Рекомендуемое использование
                                    </Text>
                                    <Text fw={500} c='blue.7'>
                                        {formatCpu(data.recommend_cpu)}
                                    </Text>
                                </Group>
                            </Stack>
                        </Stack>
                    </Paper>
                </Grid.Col>

                <Grid.Col span={6}>
                    <Paper shadow='sm' p='md' radius='md' withBorder>
                        <Stack gap='md'>
                            <Title order={4} c='blue.7'>
                                Память
                            </Title>
                            <Stack gap='xs'>
                                <Group justify='space-between'>
                                    <Text size='sm' c='dimmed'>
                                        Текущее использование
                                    </Text>
                                    <Text fw={500} c='blue.7'>
                                        {formatBytes(data.current_memory)}
                                    </Text>
                                </Group>
                                <Progress
                                    value={memoryUsagePercent}
                                    color={
                                        data.current_memory > data.recommend_memory
                                            ? 'red'
                                            : 'green'
                                    }
                                    size='md'
                                />
                            </Stack>
                            <Stack gap='xs'>
                                <Group justify='space-between'>
                                    <Text size='sm' c='dimmed'>
                                        Максимальное использование
                                    </Text>
                                    <Text fw={500} c='blue.7'>
                                        {formatBytes(data.max_memory)}
                                    </Text>
                                </Group>
                            </Stack>
                            <Stack gap='xs'>
                                <Group justify='space-between'>
                                    <Text size='sm' c='dimmed'>
                                        Рекомендуемое использование
                                    </Text>
                                    <Text fw={500} c='blue.7'>
                                        {formatBytes(data.recommend_memory)}
                                    </Text>
                                </Group>
                            </Stack>
                        </Stack>
                    </Paper>
                </Grid.Col>
            </Grid>

            <Anchor
                href={getGrafanaUrl(data.pod_name, data.namespace)}
                target='_blank'
                style={{ alignSelf: 'center' }}
            >
                <Group gap='xs'>
                    <IconChartBar size={20} />
                    <Text fw={500} c='blue.7'>
                        Открыть в Grafana
                    </Text>
                </Group>
            </Anchor>
        </Stack>
    );
}
