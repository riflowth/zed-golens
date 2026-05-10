use std::fs;
use zed_extension_api::{self as zed, Result};

struct GoLensExtension {
    cached_binary_path: Option<String>,
}

impl GoLensExtension {
    fn language_server_binary_path(
        &mut self,
        language_server_id: &zed::LanguageServerId,
    ) -> Result<String> {
        if let Some(path) = &self.cached_binary_path {
            if fs::metadata(path).is_ok_and(|stat| stat.is_file()) {
                return Ok(path.clone());
            }
        }

        let binary_path = cfg!(windows)
            .then_some("golens.exe")
            .unwrap_or("golens")
            .to_string();
        if fs::metadata(&binary_path).is_err() {
            self.build_binary(language_server_id, &binary_path)?
        }

        self.cached_binary_path = Some(binary_path.clone());
        Ok(binary_path)
    }

    fn build_binary(
        &mut self,
        language_server_id: &zed::LanguageServerId,
        output_path: &str,
    ) -> Result<()> {
        let go_path = self.find_go_binary()?;

        zed::set_language_server_installation_status(
            language_server_id,
            &zed::LanguageServerInstallationStatus::Downloading,
        );

        let (platform, arch) = zed::current_platform();
        let goos = match platform {
            zed::Os::Mac => "darwin",
            zed::Os::Linux => "linux",
            zed::Os::Windows => "windows",
        };
        let goarch = match arch {
            zed::Architecture::Aarch64 => "arm64",
            zed::Architecture::X86 => "386",
            zed::Architecture::X8664 => "amd64",
        };

        let output = std::process::Command::new(&go_path)
            .current_dir("lsp")
            .args(["build", "-o", output_path, "."])
            .env("CGO_ENABLED", "0")
            .env("GOOS", goos)
            .env("GOARCH", goarch)
            .output()
            .map_err(|e| format!("failed to run go build: {e}"))?;
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("go build failed: {stderr}"));
        }

        zed::make_file_executable(&output_path)?;
        Ok(())
    }

    fn find_go_binary(&self) -> Result<String> {
        // Lookup in common installation paths first
        let candidates = [
            "/usr/local/go/bin/go",
            "/usr/local/bin/go",
            "/opt/homebrew/bin/go",
            "C:\\Go\\bin\\go.exe",
        ];
        for candidate in &candidates {
            if fs::metadata(candidate).is_ok() {
                return Ok(candidate.to_string());
            }
        }
        // Fallback to PATH
        Ok("go".to_string())
    }
}

impl zed::Extension for GoLensExtension {
    fn new() -> Self {
        Self {
            cached_binary_path: None,
        }
    }

    fn language_server_command(
        &mut self,
        language_server_id: &zed::LanguageServerId,
        _worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        Ok(zed::Command {
            command: self.language_server_binary_path(language_server_id)?,
            args: vec![],
            env: vec![],
        })
    }
}

zed::register_extension!(GoLensExtension);
