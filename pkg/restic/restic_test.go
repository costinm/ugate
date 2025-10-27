package restic

/*
Restic is mainly CLI.

Env (or -r):
RESTIC_REPOSITORY
 - dir
 - sftp:user@host:/path
 - rclone:foo:bar

`--insecure-no-password` - no password (on encrypted disk)
RESTIC_PASSWORD

'restic init' for first time.

backup

snapshots - list of backups
  - host and path are keys

ls SNAPSHOT [--long] [PATH --recursive]
ls --host HOST latest
ls --sort size --reverse
ls --sort time


restore --target DIR snapshot



Validate: `check --read-data `

copy --from-repo NAME [SNAPSHOT]

Content of file:


dump SNAPSHOT PATH

forget SNAPSHT --keep-last N --prune

prune



restic/restic docker



 */
