import { Button } from '@/components/ui/button';

const PRESETS = [
  { label: 'Google', primary: '8.8.8.8', secondary: '8.8.4.4' },
  { label: 'Cloudflare', primary: '1.1.1.1', secondary: '1.0.0.1' },
  { label: 'Quad9', primary: '9.9.9.9', secondary: '149.112.112.112' },
] as const;

type LanDnsPresetButtonsProps = {
  onPick: (primary: string, secondary: string) => void;
};

export function LanDnsPresetButtons({ onPick }: LanDnsPresetButtonsProps) {
  return (
    <div className="flex flex-wrap gap-2">
      {PRESETS.map(({ label, primary, secondary }) => (
        <Button
          key={label}
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onPick(primary, secondary)}
        >
          {label}
        </Button>
      ))}
    </div>
  );
}
