# Prompt IPC maintenance

`promptipc` provides the single-instance control channel used by desktop prompt
imports and raise/open-result messages.

## Endpoints

- Windows keeps the loopback TCP endpoint derived from the stable data root.
- Linux and macOS normally use `<stable-data-root>/control/prompt-import.sock`.
- Unix paths longer than the shared 103-byte limit use a private deterministic
  directory under `os.TempDir`, keyed by the absolute stable data root and the
  effective UID.

## Unix ownership protocol

The Unix socket has a sibling `.lock` file. The file is opened without following
symlinks, must be a regular file owned by the effective UID, is forced to mode
`0600`, and is held with an exclusive non-blocking `flock` for the server's full
lifetime. A small in-process reservation complements the OS lock because `flock`
semantics for independently opened descriptors vary across Unix systems.

The lock file is deliberately persistent. Removing it on shutdown would allow
one process to retain a lock on an unlinked inode while another process creates
and locks a different inode at the same path. Kernel lock ownership disappears
on process exit, so a subsequent owner can safely remove a crash-stale socket.

Only the lease holder may remove a stale socket and bind a replacement. Gio's
automatic Unix-listener unlink is disabled. The server records the bound socket
inode and removes the path during shutdown only when it still names that inode.

## Shutdown contract

`Server.Close` first gates handler dispatch and marks the server closing. It then
closes the listener and every accepted connection, waits for the accept loop,
connection readers, and active handlers, removes its owned Unix socket, and only
then releases the endpoint lease. No handler starts after the closing state is
published, and no handler remains when `Close` returns. A handler must therefore
not call `Close` synchronously from inside itself.
