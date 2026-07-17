package storage

type DataDirProvider struct {
	repositories RepositoryBundle
}

func (p *DataDirProvider) Name() string { return StorageOptionDataDir }

func (p *DataDirProvider) Repositories() RepositoryBundle { return p.repositories }

func (p *DataDirProvider) SQLStore() *SQLStore { return nil }

func (p *DataDirProvider) Close() error { return nil }

type SQLiteProvider struct {
	store        *SQLiteStore
	repositories RepositoryBundle
}

func (p *SQLiteProvider) Name() string { return StorageOptionDatabase }

func (p *SQLiteProvider) Repositories() RepositoryBundle { return p.repositories }

func (p *SQLiteProvider) SQLStore() *SQLStore {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.SQLStore
}

func (p *SQLiteProvider) Close() error {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.Close()
}

type PostgresProvider struct {
	store        *PostgresStore
	repositories RepositoryBundle
}

func (p *PostgresProvider) Name() string { return StorageOptionDatabase }

func (p *PostgresProvider) Repositories() RepositoryBundle { return p.repositories }

func (p *PostgresProvider) SQLStore() *SQLStore {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.SQLStore
}

func (p *PostgresProvider) Close() error {
	if p == nil || p.store == nil {
		return nil
	}
	return p.store.Close()
}
