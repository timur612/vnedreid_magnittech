import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
    Text,
    Group,
    Badge,
    Stack,
    Anchor,
    Progress,
    Paper,
    Title,
    Grid,
    NumberInput,
    ActionIcon,
    Tooltip,
    rem,
    Divider,
    Loader,
    Center,
} from '@mantine/core';
import {
    getPodMetrics,
    getGrafanaUrl,
    updatePodLimits,
    getLlmRecommendations,
} from '../api/cluster';
import { formatBytes, formatCpu } from '../utils/format';
import type { PodMetrics } from '../types/cluster';
import { IconChartBar, IconEdit, IconCheck, IconX, IconBulb } from '@tabler/icons-react';
import { useState } from 'react';
import { notifications } from '@mantine/notifications';

interface PodDetailsProps {
    namespace: string;
    podId: string;
}

export function PodDetails({ namespace, podId }: PodDetailsProps) {
    const [isEditing, setIsEditing] = useState(false);
    const [cpuLimit, setCpuLimit] = useState(0);
    const [memoryLimit, setMemoryLimit] = useState(0);
    const queryClient = useQueryClient();

    const { data, isLoading, error } = useQuery<PodMetrics>({
        queryKey: ['podMetrics', namespace, podId],
        queryFn: () => getPodMetrics(namespace, podId),
        enabled: !!namespace && !!podId,
        retry: false,
    });

    const { data: recommendations, isLoading: isLoadingRecommendations } = useQuery({
        queryKey: ['llmRecommendations', podId],
        queryFn: () => getLlmRecommendations(podId),
        enabled: !!podId,
    });

    if (isLoading) return <Text>Загрузка...</Text>;
    if (error) return <Text c='red'>Ошибка при загрузке данных</Text>;
    if (!data) return null;

    const handleEdit = () => {
        setCpuLimit(data.recommend_cpu);
        setMemoryLimit(data.recommend_memory);
        setIsEditing(true);
    };

    const handleSave = async () => {
        try {
            await updatePodLimits(podId, namespace, cpuLimit, memoryLimit);
            notifications.show({
                title: 'Успех',
                message: 'Лимиты пода успешно обновлены',
                color: 'green',
            });
            setIsEditing(false);
            queryClient.invalidateQueries({ queryKey: ['clusterStats'] });
            queryClient.invalidateQueries({ queryKey: ['podMetrics', namespace, podId] });
        } catch (error) {
            notifications.show({
                title: 'Ошибка',
                message: 'Не удалось обновить лимиты пода',
                color: 'red',
            });
        }
    };

    const handleCancel = () => {
        setIsEditing(false);
    };

    const cpuUsagePercent = (data.max_cpu / data.current_cpu) * 100;
    const memoryUsagePercent = (data.max_memory / data.current_memory) * 100;

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
                    {!isEditing ? (
                        <Tooltip label='Редактировать лимиты'>
                            <ActionIcon variant='light' color='blue' onClick={handleEdit}>
                                <IconEdit style={{ width: rem(16), height: rem(16) }} />
                            </ActionIcon>
                        </Tooltip>
                    ) : (
                        <Group gap='xs'>
                            <Tooltip label='Сохранить'>
                                <ActionIcon variant='light' color='green' onClick={handleSave}>
                                    <IconCheck style={{ width: rem(16), height: rem(16) }} />
                                </ActionIcon>
                            </Tooltip>
                            <Tooltip label='Отмена'>
                                <ActionIcon variant='light' color='red' onClick={handleCancel}>
                                    <IconX style={{ width: rem(16), height: rem(16) }} />
                                </ActionIcon>
                            </Tooltip>
                        </Group>
                    )}
                </Group>
            </Group>

            <Grid>
                <Grid.Col span={6}>
                    <Paper shadow='sm' p='md' radius='md' withBorder>
                        <Stack gap='md'>
                            <Title order={4} c='blue.7'>
                                CPU
                            </Title>
                            {isEditing ? (
                                <NumberInput
                                    label='Лимит CPU (m)'
                                    value={cpuLimit}
                                    onChange={(value) => setCpuLimit(Number(value))}
                                    min={0}
                                    step={100}
                                />
                            ) : (
                                <>
                                    <Stack gap='xs'>
                                        <Group justify='space-between'>
                                            <Text size='sm' c='dimmed'>
                                                Текущее использование
                                            </Text>
                                            <Text fw={500} c='blue.7'>
                                                {formatCpu(data.max_cpu)}
                                            </Text>
                                        </Group>
                                        <Progress
                                            value={cpuUsagePercent}
                                            color={cpuUsagePercent < 30 ? 'red' : 'green'}
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
                                </>
                            )}
                        </Stack>
                    </Paper>
                </Grid.Col>

                <Grid.Col span={6}>
                    <Paper shadow='sm' p='md' radius='md' withBorder>
                        <Stack gap='md'>
                            <Title order={4} c='blue.7'>
                                Память
                            </Title>
                            {isEditing ? (
                                <NumberInput
                                    label='Лимит памяти (байт)'
                                    value={memoryLimit}
                                    onChange={(value) => setMemoryLimit(Number(value))}
                                    min={0}
                                    step={1024 * 1024}
                                />
                            ) : (
                                <>
                                    <Stack gap='xs'>
                                        <Group justify='space-between'>
                                            <Text size='sm' c='dimmed'>
                                                Текущее использование
                                            </Text>
                                            <Text fw={500} c='blue.7'>
                                                {formatBytes(data.max_memory)}
                                            </Text>
                                        </Group>
                                        <Progress
                                            value={memoryUsagePercent}
                                            color={memoryUsagePercent < 30 ? 'red' : 'green'}
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
                                </>
                            )}
                        </Stack>
                    </Paper>
                </Grid.Col>
            </Grid>

            <Paper shadow='sm' p='md' radius='md' withBorder>
                <Stack gap='md'>
                    <Group>
                        <IconBulb size={24} color='yellow' />
                        <Title order={4} c='yellow.7'>
                            Рекомендации AI
                        </Title>
                    </Group>
                    <Divider />
                    {isLoadingRecommendations ? (
                        <Center py='xl'>
                            <Stack align='center' gap='xs'>
                                <Loader size='sm' color='yellow' />
                                <Text size='sm' c='dimmed'>
                                    Загрузка рекомендаций...
                                </Text>
                            </Stack>
                        </Center>
                    ) : recommendations ? (
                        <Text style={{ whiteSpace: 'pre-line' }}>
                            {recommendations.recommendation}
                        </Text>
                    ) : (
                        <Text c='dimmed'>Нет рекомендаций</Text>
                    )}
                </Stack>
            </Paper>

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
