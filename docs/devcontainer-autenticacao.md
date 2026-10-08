# Autenticação persistente no DevContainer

O DevContainer monta os diretórios `~/.codex` e `~/.claude` do host em
`/home/vscode/.codex` e `/home/vscode/.claude`, com leitura e escrita.
O `initializeCommand` cria os diretórios no host antes de iniciar o container.
Essa configuração usa o host Linux/WSL e sua variável `HOME`, como a montagem SSH existente.

`CODEX_HOME` e `CLAUDE_CONFIG_DIR` apontam para esses diretórios. Credenciais,
configurações e históricos armazenados neles sobrevivem à recriação do container.
As ferramentas precisam de escrita para atualizar os tokens. O usuário `vscode`
deve ter permissão de escrita nos diretórios montados.

## Antes do primeiro rebuild com esta configuração

O diretório do Codex no container atual ainda não está montado no host.
Para aproveitar seu login, execute **no host**, com o container ainda em execução:

```sh
mkdir -p "$HOME/.codex"
# Execute a cópia somente se não houver auth.json no host.
if [ ! -e "$HOME/.codex/auth.json" ]; then
  docker cp hublinks-dev:/home/vscode/.codex/auth.json "$HOME/.codex/auth.json"
  chmod 600 "$HOME/.codex/auth.json"
fi
```

O Claude já utiliza a montagem do host; não é necessário copiar suas credenciais.
Depois, execute **Dev Containers: Rebuild Container** no VS Code.
Se não copiar o cache do Codex, faça login uma vez após o rebuild.

Para o Codex, use armazenamento em arquivo no `~/.codex/config.toml` do host:

```toml
cli_auth_credentials_store = "file"
```

Edite a opção existente, se houver, sem duplicá-la. Credenciais em keyring ou
armazenamento efêmero não são preservadas por essa montagem.

## Verificação após o rebuild

No terminal do container:

```sh
test -w "$CODEX_HOME" && test -w "$CLAUDE_CONFIG_DIR"
codex login status
claude auth status
```

Repita o rebuild e os comandos de status para verificar que o login foi mantido.
Tokens revogados ou sessões invalidadas pelo serviço ainda exigem novo login.
O logout também altera as credenciais compartilhadas com o host.

Não copie credenciais para o repositório nem para a imagem Docker.

## Solução de problemas: `Permission denied` no Codex

Sintoma: o Codex falha com `failed to create daemon state directory
/home/vscode/.codex/app-server-daemon: Permission denied (os error 13)`.

Causa: o diretório do host foi criado como `root` (por exemplo, pelo Docker, quando
`~/.codex` não existia), e o usuário `vscode` (uid 1000) não consegue gravar nele.
Confirme com `ls -ld ~/.codex`.

Correção, no terminal do container (a mudança vale também no host, pois é um bind mount):

```sh
sudo chown vscode:vscode "$CODEX_HOME"
```

O mesmo vale para `~/.claude`, se necessário. O `postCreateCommand` do
`devcontainer.json` já aplica esse `chown` (somente nos dois diretórios, sem
recursão) a cada criação do container.

Referências: [autenticação do Codex](https://developers.openai.com/codex/auth/)
e [autenticação do Claude Code](https://code.claude.com/docs/en/authentication).
