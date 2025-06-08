import { MantineProvider, createTheme, Container, Stack } from '@mantine/core';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Notifications } from '@mantine/notifications';
import { ModalsProvider } from '@mantine/modals';
import { PodsList } from './components/PodsList';
import { DeadContainers } from './components/DeadContainers';
import '@mantine/core/styles.css';
import '@mantine/notifications/styles.css';

const theme = createTheme({
    primaryColor: 'blue',
    fontFamily: 'Inter, sans-serif',
    defaultRadius: 'md',
    components: {
        Card: {
            defaultProps: {
                shadow: 'sm',
                withBorder: true,
            },
        },
        Badge: {
            defaultProps: {
                size: 'lg',
            },
        },
        Paper: {
            defaultProps: {
                shadow: 'sm',
                withBorder: true,
            },
        },
        Modal: {
            defaultProps: {
                centered: true,
                size: 'xl',
            },
        },
        Container: {
            defaultProps: {
                size: 'xl',
            },
        },
    },
});

const queryClient = new QueryClient();

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <MantineProvider theme={theme} defaultColorScheme='light'>
                <ModalsProvider>
                    <Notifications position='top-right' />
                    <Container size='xl' py='xl'>
                        <Stack gap='xl'>
                            <PodsList />
                            <DeadContainers />
                        </Stack>
                    </Container>
                </ModalsProvider>
            </MantineProvider>
        </QueryClientProvider>
    );
}

export default App;
