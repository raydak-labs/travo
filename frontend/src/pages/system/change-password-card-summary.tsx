import { Button } from '@/components/ui/button';

type ChangePasswordCardSummaryProps = {
  onEdit: () => void;
};

export function ChangePasswordCardSummary({ onEdit }: ChangePasswordCardSummaryProps) {
  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-600 dark:text-gray-300">
        Update the admin password used to log in to the router GUI.
      </p>
      <div>
        <Button size="sm" onClick={onEdit}>
          Edit Password
        </Button>
      </div>
    </div>
  );
}
