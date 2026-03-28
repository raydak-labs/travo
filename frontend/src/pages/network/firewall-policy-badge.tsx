import { Badge } from '@/components/ui/badge';

export function FirewallPolicyBadge({ policy }: { policy: string }) {
  const variant =
    policy === 'ACCEPT'
      ? 'success'
      : policy === 'DROP' || policy === 'REJECT'
        ? 'destructive'
        : 'secondary';
  return <Badge variant={variant}>{policy}</Badge>;
}
