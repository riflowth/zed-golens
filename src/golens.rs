use std::fs;

use zed_extension_api::{self as zed, Result};

const LS_BINARY: &str = "golens";
const LS_GITHUB_REPO: &str = "vectier/golens";

struct GoLensExtension {
    cached_binary_path: Option<String>,
}

impl GoLensExtension {
    fn language_server_binary_path(
        &mut self,
        language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<String> {
        if let Some(path) = worktree.which(LS_BINARY) {
            return Ok(path);
        }

        if let Some(path) = &self.cached_binary_path {
            if fs::metadata(path).is_ok_and(|stat| stat.is_file()) {
                return Ok(path.clone());
            }
        }

        zed::set_language_server_installation_status(
            language_server_id,
            &zed::LanguageServerInstallationStatus::CheckingForUpdate,
        );

        let release = zed::latest_github_release(
            LS_GITHUB_REPO,
            zed::GithubReleaseOptions {
                require_assets: false,
                pre_release: false,
            },
        )?;
        let (platform, arch) = zed::current_platform();
        let download_url = format!(
            "https://github.com/vectier/golens/releases/download/{version}/golens-{os}-{arch}.tar.gz",
            version = release.version,
            os = match platform {
                zed::Os::Linux => "linux",
                zed::Os::Mac => "darwin",
                zed::Os::Windows => "windows",
            },
            arch = match arch {
                zed::Architecture::X8664 => "amd64",
                zed::Architecture::X86 => "386",
                zed::Architecture::Aarch64 => "arm64",
            },
        );

        if !fs::metadata(LS_BINARY).is_ok_and(|stat| stat.is_file()) {
            zed::set_language_server_installation_status(
                language_server_id,
                &zed::LanguageServerInstallationStatus::Downloading,
            );
            zed::download_file(&download_url, "", zed::DownloadedFileType::GzipTar)
                .map_err(|e| format!("failed to download file {download_url}: {e}"))?;
            zed::make_file_executable(LS_BINARY)?;
        }

        self.cached_binary_path = Some(LS_BINARY.to_string());
        Ok(LS_BINARY.to_string())
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
        worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        Ok(zed::Command {
            command: self.language_server_binary_path(language_server_id, worktree)?,
            args: vec![],
            env: vec![],
        })
    }
}

zed::register_extension!(GoLensExtension);
