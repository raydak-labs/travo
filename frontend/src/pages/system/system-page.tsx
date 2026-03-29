import { NTPConfigCard } from './ntp-config-card';
import { LEDControlCard } from './led-control-card';
import { FirmwareUpgradeCard } from './firmware-upgrade-card';
import { ChangePasswordCard } from './change-password-card';
import { HardwareButtonsCard } from './hardware-buttons-card';
import { SSHKeysCard } from './ssh-keys-card';
import { AlertThresholdsCard } from './alert-thresholds-card';
import { SystemAtAGlanceSection } from './system-at-a-glance-section';
import { SystemTimezoneCard } from './system-timezone-card';
import { SystemBackupRestoreCard } from './system-backup-restore-card';
import { SystemQuickLinksCard } from './system-quick-links-card';
import { SystemPowerSection } from './system-power-section';
import { AdGuardPasswordCard } from './adguard-password-card';

export function SystemPage() {
  return (
    <div className="space-y-6">
      <SystemAtAGlanceSection />

      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Configuration
        </h2>
        <div className="space-y-4">
          <SystemTimezoneCard />
          <NTPConfigCard />
          <ChangePasswordCard />
          <AdGuardPasswordCard />
          <HardwareButtonsCard />
          <LEDControlCard />
          <AlertThresholdsCard />
          <SSHKeysCard />
        </div>
      </div>

      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Maintenance
        </h2>
        <div className="space-y-4">
          <SystemBackupRestoreCard />
          <FirmwareUpgradeCard />
        </div>
      </div>

      <SystemPowerSection />
      <SystemQuickLinksCard />
    </div>
  );
}
