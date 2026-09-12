import * as path from 'path';
import {
  commands,
  ExtensionContext,
  ExtensionMode,
  OutputChannel,
  window,
  workspace,
} from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;
let output: OutputChannel;

export async function activate(context: ExtensionContext): Promise<void> {
  output = window.createOutputChannel('Caja Language Server');
  context.subscriptions.push(output);

  context.subscriptions.push(
    commands.registerCommand('caja.restartServer', () => restart(context)),
    commands.registerCommand('caja.showOutput', () => output.show())
  );

  await start(context);
}

export function deactivate(): Thenable<void> | undefined {
  return client?.stop();
}

async function start(context: ExtensionContext): Promise<void> {
  const command = serverCommand(context);

  const serverOptions: ServerOptions = {
    command,
    args: ['lsp'],
    options: { env: process.env },
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'caja' }],
    outputChannel: output,
    // The server keeps a workspace index and re-checks open files when a module they
    // import changes on disk. It only learns about those changes if the client watches
    // for them, so without this the cross-file diagnostics never update.
    synchronize: {
      fileEvents: workspace.createFileSystemWatcher('**/*.caja'),
    },
  };

  client = new LanguageClient(
    'cajaLanguageServer',
    'Caja Language Server',
    serverOptions,
    clientOptions
  );

  try {
    await client.start();
    output.appendLine(`Caja language server started: ${command}`);
  } catch (error) {
    client = undefined;
    reportStartFailure(command, error);
  }
}

/**
 * Surfaces a failed start instead of letting it disappear.
 *
 * `client.start()` returns a promise; leaving it unawaited meant a missing binary
 * produced nothing but an unhandled rejection, and the extension simply appeared to do
 * nothing at all — which is the failure mode `editors/CLAUDE.md` warns about.
 */
function reportStartFailure(command: string, error: unknown): void {
  const detail = error instanceof Error ? error.message : String(error);
  output.appendLine(`Failed to start the Caja language server (${command}): ${detail}`);

  const configure = 'Set server path';
  const showLog = 'Show log';

  window
    .showErrorMessage(
      `Could not start the Caja language server using "${command}". ` +
        'Check that the caja CLI is installed and on your PATH, or set "caja.server.path".',
      configure,
      showLog
    )
    .then((choice) => {
      if (choice === configure) {
        commands.executeCommand('workbench.action.openSettings', 'caja.server.path');
      } else if (choice === showLog) {
        output.show();
      }
    });
}

/**
 * Resolves which binary to launch: an explicit setting wins, then the development build,
 * then whatever `caja` resolves to on PATH.
 */
function serverCommand(context: ExtensionContext): string {
  const configured = workspace.getConfiguration('caja').get<string>('server.path')?.trim();
  if (configured) {
    return configured;
  }

  if (context.extensionMode === ExtensionMode.Development) {
    // Running under F5: the binary lives in the repository's bin/ directory. Kept in the
    // output channel rather than a popup, which fired on every launch.
    const devPath = path.join(context.extensionPath, '..', '..', '..', 'bin', 'caja');
    output.appendLine(`Development mode: using ${devPath}`);
    return devPath;
  }

  return 'caja';
}

async function restart(context: ExtensionContext): Promise<void> {
  output.appendLine('Restarting the Caja language server...');

  if (client) {
    await client.stop();
    client = undefined;
  }
  await start(context);
}
