import type { ItemDetail, VerificationJob } from '../../lib/types';
import { providerLabel } from '../ai-session/providerDisplay';

export function visibleItemWarnings(plan: ItemDetail | null): { itemPath?: string; message: string }[] {
  if (!plan?.warnings?.length) return [];
  return plan.warnings.filter((warning) => !isIgnorableWarning(warning.message));
}

export function isIgnorableWarning(message: string): boolean {
  const normalized = message.toLowerCase();
  return normalized.includes("plan.yaml") && normalized.includes("does not exist in");
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

export function formatAt(value?: string): string {
  if (!value) return 'Unknown time';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function formatArtifactKind(kind: string, path: string): string {
  const fileName = path.split('/').at(-1)?.toLowerCase() ?? '';
  if (kind === 'automation_log' || fileName === 'automation.log') return 'Automation log';
  if (kind === 'verify_log' || fileName === 'verify.log') return 'Verification log';
  if (kind === 'runtime_log' || fileName === 'runtime.log') return 'Runtime setup log';
  if (kind === 'playwright_trace') return 'Playwright trace';
  if (kind === 'playwright_video') return 'Playwright video';
  if (kind === 'playwright_screenshot') return 'Screenshot';
  if (kind === 'playwright_report') return 'Test report';
  return kind.replaceAll('_', ' ');
}

export function verificationTriggerLabel(job: VerificationJob): string {
  const mode = job.terminalMode ? ` (${job.terminalMode})` : '';
  if (job.trigger?.startsWith('checkpoint:')) {
    const eventType = job.trigger.slice('checkpoint:'.length);
    return `${eventType}${mode}`;
  }
  if (job.trigger === 'rerun') {
    return `rerun${mode}`;
  }
  if (job.trigger === 'manual_checkpoint') {
    return `manual${mode}`;
  }
  return `${job.trigger || 'manual'}${mode}`;
}

export function verificationTriggerDescription(job: VerificationJob): string {
  const provider = job.provider ? ` via ${providerLabel(job.provider)}` : '';
  if (job.trigger?.startsWith('checkpoint:')) {
    const eventType = job.trigger.slice('checkpoint:'.length).replaceAll('_', ' ');
    return `Auto verification from ${eventType}${provider}${job.sessionId ? ` (session ${job.sessionId})` : ''}.`;
  }
  if (job.trigger === 'rerun') {
    return `Re-run requested${provider}${job.sessionId ? ` (session ${job.sessionId})` : ''}.`;
  }
  return `Manual verification request${provider}.`;
}

export function verificationTriggerTone(job: VerificationJob): 'manual' | 'rerun' | 'auto-embedded' | 'auto-external' | 'auto' {
  if (job.trigger === 'rerun') {
    return 'rerun';
  }
  if (job.trigger?.startsWith('checkpoint:')) {
    if (job.terminalMode === 'embedded') {
      return 'auto-embedded';
    }
    if (job.terminalMode === 'external') {
      return 'auto-external';
    }
    return 'auto';
  }
  return 'manual';
}
