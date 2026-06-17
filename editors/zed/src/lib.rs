use zed_extension_api::{self as zed, settings::LspSettings, LanguageServerId, Result};

struct RuneExtension;

impl RuneExtension {
    fn language_server_binary(
        &self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<String> {
        let configured_path = LspSettings::for_worktree(language_server_id.as_ref(), worktree)
            .ok()
            .and_then(|settings| settings.binary.and_then(|binary| binary.path));

        configured_path
            .or_else(|| worktree.which("rune-lsp"))
            .ok_or_else(|| {
                "rune-lsp binary not found. Set `lsp.rune-lsp.binary.path` or add rune-lsp to PATH"
                    .to_string()
            })
    }

    fn matches_rune_lsp(&self, language_server_id: &LanguageServerId) -> bool {
        language_server_id.as_ref() == "rune-lsp"
    }
}

impl zed::Extension for RuneExtension {
    fn new() -> Self {
        Self
    }

    fn language_server_command(
        &mut self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        if !self.matches_rune_lsp(language_server_id) {
            return Err(format!("unsupported language server: {language_server_id}"));
        }

        let binary = self.language_server_binary(language_server_id, worktree)?;
        let args = LspSettings::for_worktree(language_server_id.as_ref(), worktree)
            .ok()
            .and_then(|settings| settings.binary)
            .and_then(|binary| binary.arguments)
            .unwrap_or_default();

        Ok(zed::Command {
            command: binary,
            args,
            env: vec![],
        })
    }

    fn language_server_initialization_options(
        &mut self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<Option<zed_extension_api::serde_json::Value>> {
        let settings = LspSettings::for_worktree(language_server_id.as_ref(), worktree)
            .ok()
            .and_then(|settings| settings.initialization_options);

        Ok(settings)
    }

    fn language_server_workspace_configuration(
        &mut self,
        language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<Option<zed_extension_api::serde_json::Value>> {
        let settings = LspSettings::for_worktree(language_server_id.as_ref(), worktree)
            .ok()
            .and_then(|settings| settings.settings);

        Ok(settings)
    }
}

zed::register_extension!(RuneExtension);
