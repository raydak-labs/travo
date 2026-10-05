import { Link } from '@tanstack/react-router';

export function WireguardInstallPrompt() {
  return (
    <div className="py-4 text-center">
      {/* Only rendered once the service list or VPN status confirmed the package
          is absent; a failed request never reaches this component. */}
      <p className="mb-2 text-sm">WireGuard is not installed</p>
      <Link to="/services" className="text-sm text-blue-600 hover:underline dark:text-blue-400">
        Install via Services →
      </Link>
    </div>
  );
}
