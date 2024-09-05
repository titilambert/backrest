//! `backup` example
use rustic_backend::BackendOptions;
use rustic_core::{
    BackupOptions, ConfigOptions, KeyOptions, PathList, Repository, RepositoryOptions,
    SnapshotOptions,
};
use simplelog::{Config, LevelFilter, SimpleLogger};
use std::{error::Error, path};

fn main() -> Result<(), Box<dyn Error>> {
    // Display info logs
    let _ = SimpleLogger::init(LevelFilter::Info, Config::default());

    // Display info logs
    let _ = SimpleLogger::init(LevelFilter::Info, Config::default());

    // Initialize Backends
    let backends = BackendOptions::default()
        .repository("/tmp/repo")
        .to_backends()?;

    // Init repository
    let repo_opts = RepositoryOptions::default().password("test");
    let key_opts = KeyOptions::default();
    let config_opts = ConfigOptions::default();
    let _repo = Repository::new(&repo_opts, &backends)?.init(&key_opts, &config_opts)?;

    // Reopen
    let repo = _repo.open()?.to_indexed_ids()?;

    let backup_opts = BackupOptions::default();
    let snap = SnapshotOptions::default().to_snapshot()?;
    let path_list = PathList::from_string("/tmp/.ICE-unix")?.sanitize()?;

    // Create snapshot
    let snap = repo.backup(&backup_opts, &path_list, snap)?;

    println!("Snapshot: {:?}", snap);

    Ok(())
}
