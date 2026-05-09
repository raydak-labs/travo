import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useStatsHistory } from '@/hooks/use-stats-history';

// Computed once at module load — stable across all renders of this component.
const STATS_SINCE = Math.floor(Date.now() / 1000) - 3600;

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function StatsHistoryCard() {
  const { data: points, isLoading } = useStatsHistory(STATS_SINCE);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">System History</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-48 w-full" />
        </CardContent>
      </Card>
    );
  }

  if (!points || points.length < 2) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">System History</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Collecting data...</p>
        </CardContent>
      </Card>
    );
  }

  const chartData = points.map((p) => ({
    time: formatTime(p.time),
    cpu: Math.round(p.cpu * 10) / 10,
    memory: Math.round(p.memory * 10) / 10,
  }));

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium">System History (1h)</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={180}>
          <LineChart data={chartData} margin={{ top: 5, right: 5, left: -20, bottom: 5 }}>
            <XAxis dataKey="time" tick={{ fontSize: 10 }} interval="preserveStartEnd" />
            <YAxis domain={[0, 100]} tick={{ fontSize: 10 }} unit="%" />
            <Tooltip contentStyle={{ fontSize: 12 }} formatter={(value) => [`${value ?? ''}%`]} />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            <Line
              type="monotone"
              dataKey="cpu"
              name="CPU"
              stroke="var(--chart-1)"
              strokeWidth={1.5}
              dot={false}
            />
            <Line
              type="monotone"
              dataKey="memory"
              name="Memory"
              stroke="var(--chart-2)"
              strokeWidth={1.5}
              dot={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  );
}
