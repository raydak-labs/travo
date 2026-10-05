import { PageSection } from '@/components/ui/page-section';
import { SectionHeading } from '@/components/ui/section-heading';
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
        <SectionHeading>Configuration</SectionHeading>
        <div className="space-y-4">
          <SystemTimezoneCard />
          <NTPConfigCard />
          <ChangePasswordCard />
          <AdGuardPasswordCard />
          <HardwareButtonsCard />
          <LEDControlCard />
          <AlertThresholdsCard />
          <PageSection title="SSH Public Keys">
            <SSHKeysCard />
          </PageSection>
        </div>
      </div>

      <div>
        <SectionHeading>Maintenance</SectionHeading>
        <div className="space-y-4">
          <PageSection title="Backup & Restore">
            <SystemBackupRestoreCard />
          </PageSection>
          <PageSection title="Firmware Upgrade">
            <FirmwareUpgradeCard />
          </PageSection>
        </div>
      </div>

      <PageSection title="Danger Zone">
        <SystemPowerSection />
      </PageSection>
      <SystemQuickLinksCard />
    </div>
  );
}
