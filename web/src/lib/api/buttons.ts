import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type ButtonRun,
  type ButtonStatus,
  getButtons,
  type HardwareButton,
  setButtonAction,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type ButtonAction = HardwareButton['action'];

export const buttonActions: ButtonAction[] = ['none', 'vpn', 'guest_wifi'];

export async function fetchButtons(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<ButtonStatus> {
  const { data, response } = await getButtons({ signal, client: requestClient });
  if (!data) throw new APIRequestError(m.buttons_unavailable(), response?.status);
  return data;
}

export function actionLabel(action: ButtonAction): string {
  switch (action) {
    case 'vpn':
      return m.buttons_action_vpn();
    case 'guest_wifi':
      return m.buttons_action_guest_wifi();
    default:
      return m.buttons_action_none();
  }
}

/** The confirm text before the switch may change live state; none needs no confirm. */
export function confirmText(action: ButtonAction): string | undefined {
  switch (action) {
    case 'vpn':
      return m.buttons_confirm_vpn();
    case 'guest_wifi':
      return m.buttons_confirm_guest_wifi();
    default:
      return undefined;
  }
}

export function positionLabel(position: ButtonRun['position']): string {
  return position === 'pressed' ? m.buttons_position_pressed() : m.buttons_position_released();
}

export function outcomeLabel(outcome: ButtonRun['outcome']): string {
  switch (outcome) {
    case 'applied':
      return m.buttons_outcome_applied();
    case 'unchanged':
      return m.buttons_outcome_unchanged();
    case 'no_action':
      return m.buttons_outcome_no_action();
    case 'rolled_back':
      return m.buttons_outcome_rolled_back();
    case 'skipped_guard':
    case 'skipped_busy':
      return m.buttons_outcome_skipped();
    case 'skipped_lockout':
      return m.buttons_outcome_lockout();
    default:
      return m.buttons_outcome_failed();
  }
}

/** Maps the API's stable error codes to a message; never parses error text. */
export function buttonsError(code: string | undefined): string {
  switch (code) {
    case 'button_not_configurable':
      return m.buttons_err_not_configurable();
    case 'uci_busy':
      return m.settings_err_busy();
    default:
      return m.buttons_err_failed();
  }
}

export type Result = { status: ButtonStatus } | { error: string };

export async function saveButtonAction(name: string, action: ButtonAction): Promise<Result> {
  try {
    const { data, error } = await setButtonAction({ path: { name }, body: { action } });
    if (data) return { status: data };
    return { error: buttonsError((error as { code?: string } | undefined)?.code) };
  } catch {
    return { error: buttonsError(undefined) };
  }
}
