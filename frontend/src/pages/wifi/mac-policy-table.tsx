import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { MACPolicy } from '@shared/index';

type MACPolicyTableProps = {
  policies: MACPolicy[];
  onDelete: (index: number) => void;
  isPending: boolean;
};

export function MACPolicyTable({ policies, onDelete, isPending }: MACPolicyTableProps) {
  if (policies.length === 0) {
    return <p className="text-xs text-gray-400">No policies configured.</p>;
  }

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b text-xs text-gray-500">
          <th className="pb-1 text-left font-medium">SSID</th>
          <th className="pb-1 text-left font-medium">MAC Address</th>
          <th className="pb-1" />
        </tr>
      </thead>
      <tbody>
        {policies.map((policy, i) => (
          <tr key={`${i}-${policy.ssid}-${policy.mac ?? ''}`} className="border-b last:border-0">
            <td className="py-1.5 font-mono text-xs">{policy.ssid}</td>
            <td className="py-1.5 font-mono text-xs text-gray-600 dark:text-gray-400">
              {policy.mac || <span className="italic text-gray-400">default</span>}
            </td>
            <td className="py-1.5 text-right">
              <Button
                variant="ghost"
                size="sm"
                type="button"
                onClick={() => onDelete(i)}
                disabled={isPending}
                className="h-6 w-6 p-0 text-red-500 hover:text-red-700"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
