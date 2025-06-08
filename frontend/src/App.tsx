import { MantineProvider, createTheme } from '@mantine/core';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Notifications } from '@mantine/notifications';
import { ModalsProvider } from '@mantine/modals';
import { PodsList } from './components/PodsList';
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
                    <div
                        style={{
                            padding: '2rem',
                            backgroundColor: 'var(--mantine-color-gray-0)',
                            minHeight: '100vh',
                            width: '100%',
                            maxWidth: '1400px',
                            margin: '0 auto',
                        }}
                    >
                        <PodsList />
                    </div>
                </ModalsProvider>
            </MantineProvider>
        </QueryClientProvider>
    );
}

export default App;
