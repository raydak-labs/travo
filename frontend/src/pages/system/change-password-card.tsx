import { useState } from 'react';
import { KeyRound } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useChangePassword } from '@/hooks/use-system';

export function ChangePasswordCard() {
  const changePasswordMutation = useChangePassword();
  const [isEditing, setIsEditing] = useState(false);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const resetAndClose = () => {
    setCurrentPassword('');
    setNewPassword('');
    setConfirmPassword('');
    setIsEditing(false);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Change Password</CardTitle>
        <KeyRound className="h-4 w-4 text-gray-500" />
      </CardHeader>

      <CardContent>
        {!isEditing ? (
          <div className="space-y-3">
            <p className="text-sm text-gray-600 dark:text-gray-300">
              Update the admin password used to log in to the router GUI.
            </p>
            <div>
              <Button size="sm" onClick={() => setIsEditing(true)}>
                Edit Password
              </Button>
            </div>
          </div>
        ) : (
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (newPassword !== confirmPassword) return;
              changePasswordMutation.mutate(
                { current_password: currentPassword, new_password: newPassword },
                {
                  onSuccess: () => {
                    resetAndClose();
                  },
                },
              );
            }}
          >
            <Input
              type="password"
              placeholder="Current password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
            />
            <Input
              type="password"
              placeholder="New password (min 6 characters)"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              minLength={6}
              required
            />
            <Input
              type="password"
              placeholder="Confirm new password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              minLength={6}
              required
            />

            {newPassword && confirmPassword && newPassword !== confirmPassword && (
              <p className="text-sm text-red-500">Passwords do not match</p>
            )}

            <div className="flex gap-2 flex-wrap items-center">
              <Button
                type="submit"
                size="sm"
                disabled={
                  changePasswordMutation.isPending ||
                  !currentPassword ||
                  !newPassword ||
                  newPassword !== confirmPassword ||
                  newPassword.length < 6
                }
              >
                {changePasswordMutation.isPending ? 'Changing…' : 'Change Password'}
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={resetAndClose}>
                Cancel
              </Button>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
