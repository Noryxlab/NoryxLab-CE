import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router';
import { TooltipProvider } from '@/components/ui/tooltip';
import { ToastProvider } from '@/components/ui/toast';
import { I18nProvider } from '@/lib/i18n';
import { ThemeProvider } from '@/lib/theme';
import { AuthProvider } from '@/lib/auth';
import { ApiError } from '@/lib/api/client';
import { AppRoutes } from './routes';
import { RootErrorBoundary } from './error-boundary';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 15_000,
      refetchOnWindowFocus: false,
      retry: (failureCount, error) => {
        // Retrying a 403 or a 404 only delays the error the user needs to see.
        if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
        return failureCount < 2;
      },
    },
    mutations: { retry: false },
  },
});

export function App() {
  return (
    <RootErrorBoundary>
      <I18nProvider>
        <ThemeProvider>
          <QueryClientProvider client={queryClient}>
            <AuthProvider>
              <TooltipProvider delayDuration={300}>
                <ToastProvider>
                  <BrowserRouter>
                    <AppRoutes />
                  </BrowserRouter>
                </ToastProvider>
              </TooltipProvider>
            </AuthProvider>
          </QueryClientProvider>
        </ThemeProvider>
      </I18nProvider>
    </RootErrorBoundary>
  );
}
