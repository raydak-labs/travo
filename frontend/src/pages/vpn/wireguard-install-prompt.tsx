import { Link } from '@tanstack/react-router';

export function WireguardInstallPrompt() {
  return (
    <div className="py-4 text-center">
      <p className="mb-2 text-sm text-gray-500">WireGuard is not installed</p>
      <Link to="/services" className="text-sm text-blue-600 hover:underline dark:text-blue-400">
        Install via Services →
      </Link>
    </div>
  );
}
