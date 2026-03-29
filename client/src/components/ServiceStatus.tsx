import { useEffect, useState } from 'react';
import { WifiOff } from 'lucide-react';

export default function ServiceStatus() {
  const [offline, setOffline] = useState(!navigator.onLine);
  const [backendDown, setBackendDown] = useState(false);

  useEffect(() => {
    const goOnline = () => setOffline(false);
    const goOffline = () => setOffline(true);
    window.addEventListener('online', goOnline);
    window.addEventListener('offline', goOffline);
    return () => {
      window.removeEventListener('online', goOnline);
      window.removeEventListener('offline', goOffline);
    };
  }, []);

  useEffect(() => {
    const check = () => {
      fetch('/health', { signal: AbortSignal.timeout(3000) })
        .then((r) => setBackendDown(!r.ok))
        .catch(() => setBackendDown(true));
    };
    check();
    const interval = setInterval(check, 30000);
    return () => clearInterval(interval);
  }, []);

  if (!offline && !backendDown) return null;

  return (
    <div className="fixed top-0 left-0 right-0 z-50 alert-enter">
      <div className="bg-red-600 text-white px-4 py-2 text-center text-sm font-medium flex items-center justify-center gap-2">
        <WifiOff className="w-4 h-4" />
        {offline
          ? 'You are offline. Some features may be unavailable.'
          : 'Unable to reach the server. Retrying...'}
      </div>
    </div>
  );
}
