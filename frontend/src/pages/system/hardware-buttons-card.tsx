import { useMemo, useState } from 'react';
import { ToggleLeft } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useHardwareButtons, useSetButtonActions } from '@/hooks/use-system';
import type { ButtonAction } from '@shared/index';

function actionLabel(action: ButtonAction): string {
  switch (action) {
    case 'none':
      return 'Do nothing';
    case 'vpn_toggle':
      return 'Toggle VPN';
    case 'wifi_toggle':
      return 'Toggle WiFi';
    case 'led_toggle':
      return 'Toggle LEDs';
    case 'reboot':
      return 'Reboot';
    default:
      return action;
  }
}

export function HardwareButtonsCard() {
  const { data: hardwareButtons = [] } = useHardwareButtons();
  const setButtonActions = useSetButtonActions();
  const [isEditing, setIsEditing] = useState(false);
  const [pendingButtonActions, setPendingButtonActions] = useState<Record<string, ButtonAction>>(
    {},
  );

  const resolvedButtons = useMemo(() => {
    return hardwareButtons.map((btn) => {
      const resolved = pendingButtonActions[btn.name];
      return {
        ...btn,
        action: (resolved ?? btn.action) as ButtonAction,
      };
    });
  }, [hardwareButtons, pendingButtonActions]);

  if (hardwareButtons.length === 0) return null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Hardware Buttons</CardTitle>
        <ToggleLeft className="h-4 w-4 text-gray-500" />
      </CardHeader>

      <CardContent className="space-y-4">
        <p className="text-xs text-gray-500">
          Configure what each physical button does when pressed.
        </p>

        {!isEditing ? (
          <div className="space-y-3">
            {hardwareButtons.map((btn) => (
              <div key={btn.name} className="flex items-center justify-between gap-4">
                <span className="font-mono text-sm capitalize">{btn.name}</span>
                <span className="text-sm text-gray-900 dark:text-white">
                  {actionLabel(btn.action)}
                </span>
              </div>
            ))}

            <Button
              size="sm"
              disabled={setButtonActions.isPending}
              onClick={() => setIsEditing(true)}
              title="Edit button-to-action mapping"
            >
              Edit Button Actions
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            {resolvedButtons.map((btn) => (
              <div key={btn.name} className="flex items-center justify-between gap-4">
                <span className="font-mono text-sm capitalize">{btn.name}</span>
                <Select
                  value={btn.action}
                  onValueChange={(val) =>
                    setPendingButtonActions((prev) => ({
                      ...prev,
                      [btn.name]: val as ButtonAction,
                    }))
                  }
                >
                  <SelectTrigger className="w-44">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Do nothing</SelectItem>
                    <SelectItem value="vpn_toggle">Toggle VPN</SelectItem>
                    <SelectItem value="wifi_toggle">Toggle WiFi</SelectItem>
                    <SelectItem value="led_toggle">Toggle LEDs</SelectItem>
                    <SelectItem value="reboot">Reboot</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            ))}

            <div className="flex gap-2 flex-wrap items-center">
              <Button
                size="sm"
                disabled={
                  setButtonActions.isPending || Object.keys(pendingButtonActions).length === 0
                }
                onClick={() => {
                  const merged = hardwareButtons.map((btn) => ({
                    name: btn.name,
                    action: (pendingButtonActions[btn.name] ?? btn.action) as ButtonAction,
                  }));

                  setButtonActions.mutate(
                    { buttons: merged },
                    {
                      onSuccess: () => {
                        setPendingButtonActions({});
                        setIsEditing(false);
                      },
                    },
                  );
                }}
              >
                {setButtonActions.isPending ? 'Saving…' : 'Save Button Actions'}
              </Button>

              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => {
                  setPendingButtonActions({});
                  setIsEditing(false);
                }}
                disabled={setButtonActions.isPending}
              >
                Cancel
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
