import { MantineProvider } from '@mantine/core';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { PodsList } from './components/PodsList';

const queryClient = new QueryClient();

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <MantineProvider>
                <div style={{ padding: '2rem' }}>
                    <PodsList />
                </div>
            </MantineProvider>
        </QueryClientProvider>
    );
}

export default App;
