type WireguardStatusDetailTextProps = {
  statusDetail: string | undefined;
  isToggling: boolean;
};

export function WireguardStatusDetailText({
  statusDetail,
  isToggling,
}: WireguardStatusDetailTextProps) {
  if (!statusDetail || isToggling) {
    return null;
  }

  return (
    <div className="text-xs text-gray-500">
      {statusDetail === 'disabled' && 'WireGuard is disabled.'}
      {statusDetail === 'configured' && 'WireGuard is configured but not connected yet.'}
      {statusDetail === 'enabled_not_up' &&
        'WireGuard is enabled but the interface is not up yet.'}
      {statusDetail === 'up_no_handshake' &&
        'WireGuard interface is up, but handshake has not completed yet.'}
    </div>
  );
}
