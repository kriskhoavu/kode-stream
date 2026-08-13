import { useCallback, useEffect, useState } from 'react';
import { api } from '../../shared/api';
import type { AICapability, AISettings } from '../../lib/types';

export function useAISettings() {
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [capabilities, setCapabilities] = useState<AICapability[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError('');
    setSaved(false);
    try {
			const [nextSettings, nextCapabilities] = await Promise.all([api.aiSettings(signal), api.aiCapabilities(signal)]);
      setSettings(nextSettings);
      setCapabilities(nextCapabilities);
		} catch (caught) {
			if (signal?.aborted) return;
      setError(caught instanceof Error ? caught.message : 'AI settings are unavailable.');
    } finally {
			if (!signal?.aborted) setLoading(false);
    }
  }, []);

	useEffect(() => { const controller = new AbortController(); void refresh(controller.signal); return () => controller.abort(); }, [refresh]);

  const save = useCallback(async () => {
    if (!settings) return;
    setSaving(true);
    setError('');
    setSaved(false);
    try {
      setSettings(await api.saveAISettings(settings));
      setCapabilities(await api.aiCapabilities());
      setSaved(true);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'AI settings could not be saved.');
    } finally {
      setSaving(false);
    }
  }, [settings]);

  return { settings, setSettings, capabilities, loading, saving, error, saved, refresh, save };
}
