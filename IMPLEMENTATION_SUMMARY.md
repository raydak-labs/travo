# WiFi Mode UX and Safety Improvements

## Summary

This branch implements critical safety improvements to WiFi mode switching, preventing connection loss and crash loops while disabling the repeater mode in the UI.

## Changes Made

### Frontend Changes

#### 1. Disabled Repeater Mode (UI Only)
- **File:** `frontend/src/components/wifi/wifi-mode-options.ts`
- Removed the repeater option from `WIFI_MODE_OPTIONS` array
- Removed `Repeat` icon import
- Users can now only switch between AP and Client modes

#### 2. Simplified Mode Card
- **File:** `frontend/src/components/wifi/wifi-mode-card.tsx`
- Removed RepeaterWizard import and state
- Simplified click handler to always open confirmation dialog
- Cleaner flow without special case for repeater mode

#### 3. Enhanced Safety Warnings
- **File:** `frontend/src/components/wifi/wifi-mode-switch-dialog.tsx`
- Added comprehensive two-part warning system:
  - **Amber warning:** Explains automatic rollback protection (30-second window)
  - **Blue warning:** User guidance to keep page open and handle rollback scenario
- Clear instruction on what to do if rollback occurs (refresh page)

### Backend Changes

#### 1. API Handler Validation
- **File:** `backend/internal/api/wifi_handlers.go`
- Added explicit rejection of repeater mode requests
- Returns clear error message directing users to AP or client mode
- Backend still supports repeater internally (for future use)

#### 2. Crash Guard Mechanism
- **File:** `backend/internal/services/wifi_service.go`
- Added `modeChangeGuardFile` constant (`/etc/travo/mode-change-in-progress`)
- Guard file written before `stageWirelessApply()` with session token
- Guard file cleaned up on successful confirmation
- Prevents retry loops if router crashes during mode switch

#### 3. Improved SetMode Safety
- **File:** `backend/internal/services/wifi_service.go`
- Added upfront mode validation (supports all three modes internally)
- Enhanced documentation explaining crash guard and rollback protection
- Split result handling to write guard file only after successful apply staging
- Guard file only written when token exists (production mode)

#### 4. Startup Guard Cleanup
- **File:** `backend/cmd/server/main.go`
- Added `checkAndClearModeChangeGuard()` function
- Runs on startup (not in mock mode) to clean up stale guard files
- Logs warning when guard file found (indicates previous crash)
- Prevents infinite crash-retry loops

## Safety Mechanisms

### Rollback Protection
The system uses OpenWRT's built-in `uci apply` with rollback:
1. Changes are staged with 30-second rollback window
2. Browser must confirm router is still reachable
3. If confirmation fails (timeout), config automatically reverts
4. No need for manual intervention if things go wrong

### Crash Guard Pattern
Guard file at `/etc/travo/mode-change-in-progress`:
1. Written before initiating apply+confirm
2. Contains session token for tracking
3. Removed on successful confirmation
4. Cleared on startup if stale (indicates crash)
5. Prevents automatic retry of crashed mode change

### User Experience Flow
1. User clicks mode change
2. Confirmation dialog shows detailed safety warnings
3. Change is staged with rollback timer
4. Browser polls for confirmation
5. If successful: mode updates, guard file cleaned up
6. If failed: config reverts, user sees connection error
7. User can refresh and try again safely

## Testing Considerations

### Existing Tests
- `TestWifiSetMode` - Basic mode switching
- `TestWifiSetMode_ClientDisablesAPsAndEnablesSTA` - Client mode behavior
- `TestWifiSetMode_RepeaterEnablesSTAAndAPs` - Repeater mode (still works internally)

### New Safety Tests to Add (Future)
1. Test crash guard file creation and cleanup
2. Test startup guard cleanup
3. Test mode validation (including rejected repeater via API)
4. Test rollback scenario (requires integration test with rpcd)

## Migration Notes

### For Users
- Repeater mode is no longer available in UI
- Existing repeater configurations will continue to work
- Mode changes are now safer with automatic rollback
- Better guidance on what to do if changes fail

### For Developers
- Backend still supports repeater mode for future use
- API validates mode at handler level
- Guard file pattern can be reused for other dangerous operations
- Apply+confirm flow is standard for all wireless mutations

## Files Modified
- `frontend/src/components/wifi/wifi-mode-options.ts` (removed repeater)
- `frontend/src/components/wifi/wifi-mode-card.tsx` (simplified flow)
- `frontend/src/components/wifi/wifi-mode-switch-dialog.tsx` (enhanced warnings)
- `backend/internal/api/wifi_handlers.go` (mode validation)
- `backend/internal/services/wifi_service.go` (crash guard, improved safety)
- `backend/cmd/server/main.go` (startup guard cleanup)

## Branch Information
- **Branch:** `improve-wifi-ux-safety`
- **Worktree:** `/tmp/travo-improve-wifi-ux-safety`
- **Base:** `origin/main`