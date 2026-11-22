import { type RuntimeMode } from './workspace';

export interface SystemConfigPaths {
  dataDir: string;
  defaultDataDir: string;
  cloneRootDir: string;
  registryFile?: string;
  dataDirEnvironmentLocked?: boolean;
  restartRequired?: boolean;
}

export type StorageOption = 'database' | 'datadir';
export type StorageDriver = 'file' | 'sqlite' | 'postgres';
export type StorageSyncDirection = 'datadir_to_database' | 'database_to_datadir';

export interface DatabaseHealth {
  driver: StorageDriver;
  ok: boolean;
  migrationVersion: number;
  error?: string;
}

export interface StorageStatus {
  mode: RuntimeMode;
  storageOption: StorageOption;
  storageDriver: StorageDriver;
  environmentLocked: boolean;
  storageOptionEnv: string;
  storageDriverEnv: string;
  dataDir: string;
  databasePath?: string;
  databaseUrlConfigured: boolean;
  database?: DatabaseHealth;
}

export interface StorageSyncResult {
  ok: boolean;
  direction: StorageSyncDirection;
  backupPath: string;
  summary: Record<string, number>;
  warnings: string[];
  skippedStores: string[];
}
