import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { PortForwardRule } from '@shared/index';

type FirewallPortForwardRulesTableProps = {
  rules: PortForwardRule[];
  deleteRule: { mutate: (id: string) => void; isPending: boolean };
};

export function FirewallPortForwardRulesTable({
  rules,
  deleteRule,
}: FirewallPortForwardRulesTableProps) {
  if (rules.length === 0) return null;

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-gray-500">
            <th className="pb-2 font-medium">Name</th>
            <th className="pb-2 font-medium">Protocol</th>
            <th className="pb-2 font-medium">Ext. Port</th>
            <th className="pb-2 font-medium">Internal IP</th>
            <th className="pb-2 font-medium">Int. Port</th>
            <th className="w-16 pb-2 font-medium"></th>
          </tr>
        </thead>
        <tbody>
          {rules.map((rule) => (
            <tr key={rule.id} className="border-b last:border-0">
              <td className="py-2 text-gray-900 dark:text-white">{rule.name}</td>
              <td className="py-2 font-mono uppercase text-gray-500">{rule.protocol}</td>
              <td className="py-2 font-mono text-gray-900 dark:text-white">{rule.src_dport}</td>
              <td className="py-2 font-mono text-gray-900 dark:text-white">{rule.dest_ip}</td>
              <td className="py-2 font-mono text-gray-900 dark:text-white">{rule.dest_port}</td>
              <td className="py-2 text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  type="button"
                  onClick={() => deleteRule.mutate(rule.id)}
                  disabled={deleteRule.isPending}
                >
                  <Trash2 className="h-4 w-4 text-red-500" />
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
