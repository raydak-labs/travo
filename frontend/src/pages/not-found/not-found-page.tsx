import { Link } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

export function NotFoundPage() {
  return (
    <Card className="mx-auto max-w-lg">
      <CardHeader>
        <CardTitle>Page not found</CardTitle>
        <CardDescription>
          That address does not match any page in Travo. It may have been renamed, or the link
          that brought you here may be out of date.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        <Button asChild>
          <Link to="/dashboard">Go to Dashboard</Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/logs">Check Logs</Link>
        </Button>
      </CardContent>
    </Card>
  );
}
