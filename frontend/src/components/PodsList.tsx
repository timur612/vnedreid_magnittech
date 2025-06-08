import { useQuery } from '@tanstack/react-query';
import {
    Table,
    Text,
    Badge,
    Button,
    Card,
    Group,
    Stack,
    Title,
    Paper,
    Progress,
    Tooltip,
    ActionIcon,
    rem,
    Container,
    Divider,
} from '@mantine/core';
import { useState } from 'react';
import { getClusterStats, getGrafanaUrl } from '../api/cluster';
import { formatBytes, formatCpu, formatRubles } from '../utils/format';
import { PodDetails } from './PodDetails';
import type { PodMetrics } from '../types/cluster';
import { IconChartBar, IconInfoCircle } from '@tabler/icons-react';

export function PodsList() {
    const [selectedPod, setSelectedPod] = useState<PodMetrics | null>(null);
    const { data, isLoading, error } = useQuery({
        queryKey: ['clusterStats'],
        queryFn: getClusterStats,
        refetchInterval: false,
        staleTime: Infinity,
    });

    if (isLoading) return <Text>Загрузка...</Text>;
    if (error) return <Text c='red'>Ошибка при загрузке данных</Text>;
    if (!data) return null;

    return (
        <Container size='xl'>
            <Stack gap='xl'>
                <Title order={1} ta='center' mb='xl' c='blue.7'>
                    Оптимизация ресурсов Kubernetes
                </Title>

                <Paper shadow='sm' p='xl' radius='md' withBorder>
                    <Title order={2} mb='lg' c='blue.7'>
                        Общая статистика кластера
                    </Title>
                    <Group grow>
                        <Card withBorder>
                            <Stack gap='xs'>
                                <Text size='sm' c='dimmed'>
                                    Всего подов
                                </Text>
                                <Text size='xl' fw={700} c='blue.7'>
                                    {data.total_pods}
                                </Text>
                            </Stack>
                        </Card>
                        <Card withBorder>
                            <Stack gap='xs'>
                                <Text size='sm' c='dimmed'>
                                    Текущие лимиты CPU
                                </Text>
                                <Text size='xl' fw={700} c='blue.7'>
                                    {formatCpu(data.total_current_cpu)}
                                </Text>
                                <Progress
                                    value={(data.total_current_cpu / data.total_max_cpu) * 100}
                                    color={
                                        data.total_current_cpu > data.total_recommend_cpu
                                            ? 'red'
                                            : 'green'
                                    }
                                    size='md'
                                />
                            </Stack>
                        </Card>
                        <Card withBorder>
                            <Stack gap='xs'>
                                <Text size='sm' c='dimmed'>
                                    Текущие лимиты памяти
                                </Text>
                                <Text size='xl' fw={700} c='blue.7'>
                                    {formatBytes(data.total_current_memory)}
                                </Text>
                                <Progress
                                    value={
                                        (data.total_current_memory / data.total_max_memory) * 100
                                    }
                                    color={
                                        data.total_current_memory > data.total_recommend_memory
                                            ? 'red'
                                            : 'green'
                                    }
                                    size='md'
                                />
                            </Stack>
                        </Card>
                        <Card withBorder>
                            <Stack gap='xs'>
                                <Text size='sm' c='dimmed'>
                                    Потенциальная экономия
                                </Text>
                                <Text size='xl' fw={700} c='blue.7'>
                                    {formatRubles(data.potential_savings)}
                                </Text>
                            </Stack>
                        </Card>
                    </Group>
                </Paper>

                <Paper shadow='sm' p='xl' radius='md' withBorder>
                    <Group justify='space-between' mb='lg'>
                        <Title order={2} c='blue.7'>
                            Список подов
                        </Title>
                        <Tooltip label='Score показывает эффективность использования ресурсов. Зеленый - хорошо, красный - требует оптимизации'>
                            <ActionIcon variant='subtle' color='gray'>
                                <IconInfoCircle style={{ width: rem(20), height: rem(20) }} />
                            </ActionIcon>
                        </Tooltip>
                    </Group>
                    <Table striped highlightOnHover>
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
                            {data.pods
                                .filter(
                                    (pod) => (pod.max_cpu ?? 0) > 0 && (pod.max_memory ?? 0) > 0
                                )
                                .map((pod) => (
                                    <tr
                                        key={pod.pod_name}
                                        style={{
                                            backgroundColor:
                                                selectedPod?.pod_name === pod.pod_name
                                                    ? 'var(--mantine-color-blue-0)'
                                                    : undefined,
                                            cursor: 'pointer',
                                        }}
                                        onClick={() => setSelectedPod(pod)}
                                    >
                                        <td>
                                            <Text fw={500}>{pod.pod_name}</Text>
                                        </td>
                                        <td>
                                            <Badge variant='light' color='blue'>
                                                {pod.namespace}
                                            </Badge>
                                        </td>
                                        <td>
                                            <Stack gap={4}>
                                                <Text size='sm'>
                                                    {formatCpu(pod.max_cpu)} /{' '}
                                                    {formatCpu(pod.current_cpu)}
                                                </Text>
                                                <Progress
                                                    value={(pod.max_cpu / pod.current_cpu) * 100}
                                                    color={
                                                        (pod.max_cpu / pod.current_cpu) * 100 < 30
                                                            ? 'red'
                                                            : 'green'
                                                    }
                                                    size='sm'
                                                />
                                            </Stack>
                                        </td>
                                        <td>
                                            <Stack gap={4}>
                                                <Text size='sm'>
                                                    {formatBytes(pod.max_memory)} /{' '}
                                                    {formatBytes(pod.current_memory)}
                                                </Text>
                                                <Progress
                                                    value={
                                                        (pod.max_memory / pod.current_memory) * 100
                                                    }
                                                    color={
                                                        (pod.max_memory / pod.current_memory) *
                                                            100 <
                                                        30
                                                            ? 'red'
                                                            : 'green'
                                                    }
                                                    size='sm'
                                                />
                                            </Stack>
                                        </td>
                                        <td>
                                            <Badge
                                                color={
                                                    pod.optimization_score > 0.7 ? 'green' : 'red'
                                                }
                                                variant='light'
                                            >
                                                {pod.optimization_score.toFixed(2)}
                                            </Badge>
                                        </td>
                                        <td>
                                            <Group gap='xs'>
                                                <Tooltip label='Открыть в Grafana'>
                                                    <ActionIcon
                                                        variant='light'
                                                        color='blue'
                                                        component='a'
                                                        href={getGrafanaUrl(
                                                            pod.pod_name,
                                                            pod.namespace
                                                        )}
                                                        target='_blank'
                                                        onClick={(e) => e.stopPropagation()}
                                                    >
                                                        <IconChartBar
                                                            style={{
                                                                width: rem(16),
                                                                height: rem(16),
                                                            }}
                                                        />
                                                    </ActionIcon>
                                                </Tooltip>
                                            </Group>
                                        </td>
                                    </tr>
                                ))}
                        </tbody>
                    </Table>
                </Paper>

                {selectedPod && (
                    <Paper shadow='sm' p='xl' radius='md' withBorder>
                        <Group justify='space-between' mb='lg'>
                            <Title order={2} c='blue.7'>
                                Детали пода: {selectedPod.pod_name}
                            </Title>
                            <Button
                                variant='light'
                                color='gray'
                                onClick={() => setSelectedPod(null)}
                            >
                                Закрыть
                            </Button>
                        </Group>
                        <Divider mb='lg' />
                        <PodDetails
                            namespace={selectedPod.namespace}
                            podId={selectedPod.pod_name}
                        />
                    </Paper>
                )}
            </Stack>
        </Container>
    );
}
