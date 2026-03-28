import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import type { LEDStatus } from '@shared/index';

type LedStealthStatusPanelProps = {
  ledStatus: LEDStatus;
  stealthPending: boolean;
  onToggleStealth: () => void;
};

export function LedStealthStatusPanel({
  ledStatus,
  stealthPending,
  onToggleStealth,
}: LedStealthStatusPanelProps) {
  return (
    <>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-700 dark:text-gray-300">
            {ledStatus.stealth_mode
              ? 'All LEDs are off — stealth mode active'
              : `${ledStatus.led_count} LED${ledStatus.led_count > 1 ? 's' : ''} active`}
          </p>
        </div>
        <Button
          size="sm"
          variant={ledStatus.stealth_mode ? 'default' : 'outline'}
          disabled={stealthPending}
          type="button"
          onClick={onToggleStealth}
        >
          {ledStatus.stealth_mode ? 'Restore LEDs' : 'Go Stealth'}
        </Button>
      </div>

      {ledStatus.leds && ledStatus.leds.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-medium text-gray-500">Individual LEDs</p>
          <div className="grid gap-1">
            {ledStatus.leds.map((led) => (
              <div
                key={led.name}
                className="flex items-center justify-between rounded-md bg-gray-50 px-3 py-1.5 dark:bg-gray-900"
              >
                <span className="text-xs text-gray-700 dark:text-gray-300">{led.name}</span>
                <Badge variant={led.brightness > 0 ? 'success' : 'secondary'}>
                  {led.brightness > 0 ? 'On' : 'Off'}
                </Badge>
              </div>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
