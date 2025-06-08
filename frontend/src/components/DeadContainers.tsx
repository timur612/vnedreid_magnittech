import { useQuery } from '@tanstack/react-query';
import { Paper, Title, Table, Text, Badge, Group, Stack } from '@mantine/core';
import { getDeadContainers } from '../api/cluster';
import { formatBytes } from '../utils/format';
import { IconAlertCircle } from '@tabler/icons-react';

export function DeadContainers() {
    const { data, isLoading, error } = useQuery({
        queryKey: ['deadContainers'],
        queryFn: getDeadContainers,
        refetchInterval: 30000,
    });

    if (isLoading) return <Text>Загрузка...</Text>;
    if (error) return <Text c='red'>Ошибка при загрузке данных</Text>;
    if (!data || data.length === 0) return null;

    return (
        <Paper shadow='sm' p='md' radius='md' withBorder>
            <Stack gap='md'>
                <Group>
                    <IconAlertCircle size={24} color='red' />
                    <Title order={3} c='red.7'>
                        Мертвые контейнеры
                    </Title>
                </Group>

                <Table>
                    <Table.Thead>
                        <Table.Tr>
                            <Table.Th>Под</Table.Th>
                            <Table.Th>Namespace</Table.Th>
                            <Table.Th>Контейнер</Table.Th>
                            <Table.Th>Тип</Table.Th>
                            <Table.Th>Последняя активность</Table.Th>
                            <Table.Th>Сеть (входящая)</Table.Th>
                            <Table.Th>Сеть (исходящая)</Table.Th>
                        </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                        {data.map((container) => (
                            <Table.Tr
                                key={`${container.namespace}-${container.pod_name}-${container.container_name}`}
                            >
                                <Table.Td>
                                    <Text fw={500}>{container.pod_name}</Text>
                                </Table.Td>
                                <Table.Td>
                                    <Badge variant='light' color='blue'>
                                        {container.namespace}
                                    </Badge>
                                </Table.Td>
                                <Table.Td>{container.container_name}</Table.Td>
                                <Table.Td>
                                    <Badge variant='light' color='gray'>
                                        {container.pod_type}
                                    </Badge>
                                </Table.Td>
                                <Table.Td>
                                    {container.last_activity === '0001-01-01T00:00:00Z' ? (
                                        <Text c='dimmed'>Нет данных</Text>
                                    ) : (
                                        new Date(container.last_activity).toLocaleString()
                                    )}
                                </Table.Td>
                                <Table.Td>{formatBytes(container.network_in_bytes)}</Table.Td>
                                <Table.Td>{formatBytes(container.network_out_bytes)}</Table.Td>
                            </Table.Tr>
                        ))}
                    </Table.Tbody>
                </Table>
            </Stack>
        </Paper>
    );
}
