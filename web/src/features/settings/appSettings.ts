import { useEffect, useState } from 'react';
import { statusOrder } from '../../shared/api';
import type { ItemStatus } from '../../lib/types';
import { readPreference, writePreference } from '../../shared/preferences/store';

const storageKey = 'planManager.appSettings';

export interface AppSettings {
  visibleWorkstreamStatuses: ItemStatus[];
}

export const defaultAppSettings: AppSettings = {
  visibleWorkstreamStatuses: [...statusOrder]
};

export function useAppSettings(): [AppSettings, (settings: AppSettings) => void] {
  const [settings, setSettingsState] = useState(loadAppSettings);

  useEffect(() => {
    writePreference(storageKey, settings);
  }, [settings]);

  const setSettings = (next: AppSettings) => setSettingsState(normalizeAppSettings(next));

  return [settings, setSettings];
}

export function loadAppSettings(): AppSettings {
  try {
    return readPreference(storageKey, (value) => normalizeAppSettings(value as Partial<AppSettings>), defaultAppSettings);
  } catch {
    return defaultAppSettings;
  }
}

function normalizeAppSettings(settings: Partial<AppSettings>): AppSettings {
  const visible = new Set(settings.visibleWorkstreamStatuses);
  const visibleWorkstreamStatuses = statusOrder.filter((status) => visible.has(status));
  return {
    visibleWorkstreamStatuses: visibleWorkstreamStatuses.length > 0 ? visibleWorkstreamStatuses : [...statusOrder]
  };
}
