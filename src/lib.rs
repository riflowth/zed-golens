use zed_extension_api::{self as zed, Result};

struct GoLensExtension;

impl GoLensExtension {}

impl zed::Extension for GoLensExtension {
    fn new() -> Self {
        Self
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        let lsp_proxy = worktree
            .which("golens")
            .ok_or("golens binary not found in PATH")?;

        Ok(zed::Command {
            command: lsp_proxy,
            args: vec![],
            env: vec![],
        })
    }
}

zed::register_extension!(GoLensExtension);
