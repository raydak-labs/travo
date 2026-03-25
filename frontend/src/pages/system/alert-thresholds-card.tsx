import { useState, useEffect } from 'react';
import { Bell } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useAlertThresholds, useSetAlertThresholds } from '@/hooks/use-system';

function ThresholdSlider({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <div className="space-y-2">
      <div className="flex justify-between text-sm">
        <span className="text-gray-700 dark:text-gray-300">{label}</span>
        <span className="font-medium">{value}%</span>
      </div>
      <input
        type="range"
        min={50}
        max={99}
        step={1}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full accent-blue-600"
      />
    </div>
  );
}

export function AlertThresholdsCard() {
  const { data, isLoading } = useAlertThresholds();
  const setThresholds = useSetAlertThresholds();

  const [storage, setStorage] = useState(90);
  const [cpu, setCpu] = useState(90);
  const [memory, setMemory] = useState(90);

  useEffect(() => {
    if (data) {
      setStorage(data.storage_percent);
      setCpu(data.cpu_percent);
      setMemory(data.memory_percent);
    }
  }, [data]);

  function handleSave() {
    setThresholds.mutate({
      storage_percent: storage,
      cpu_percent: cpu,
      memory_percent: memory,
    });
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Bell className="h-5 w-5" />
          Alert Thresholds
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
          </div>
        ) : (
          <>
            <ThresholdSlider label="Storage %" value={storage} onChange={setStorage} />
            <ThresholdSlider label="CPU %" value={cpu} onChange={setCpu} />
            <ThresholdSlider label="Memory %" value={memory} onChange={setMemory} />

            <Button
              onClick={handleSave}
              disabled={setThresholds.isPending}
              size="sm"
            >
              {setThresholds.isPending ? 'Saving…' : 'Save Thresholds'}
            </Button>
          </>
        )}
      </CardContent>
    </Card>
  );
}
