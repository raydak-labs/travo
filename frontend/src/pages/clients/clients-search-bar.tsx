import { Search, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

type ClientsSearchBarProps = {
  value: string;
  onChange: (value: string) => void;
};

export function ClientsSearchBar({ value, onChange }: ClientsSearchBarProps) {
  return (
    <div className="mb-3 flex items-center gap-2">
      <Search className="h-4 w-4 shrink-0 text-gray-400" />
      <Input
        placeholder="Search by name, IP, or MAC…"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="h-8 text-sm"
      />
      {value ? (
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => onChange('')}>
          <X className="h-4 w-4" />
        </Button>
      ) : null}
    </div>
  );
}
