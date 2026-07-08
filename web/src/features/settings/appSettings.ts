import { useEffect, useState } from 'react';
import { statusOrder } from '../../lib/api';
import type { ItemStatus } from '../../lib/types';

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
    localStorage.setItem(storageKey, JSON.stringify(settings));
  }, [settings]);

  const setSettings = (next: AppSettings) => setSettingsState(normalizeAppSettings(next));

  return [settings, setSettings];
}

export function loadAppSettings(): AppSettings {
  try {
    return normalizeAppSettings(JSON.parse(localStorage.getItem(storageKey) ?? '{}') as Partial<AppSettings>);
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
