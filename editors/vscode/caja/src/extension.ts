import * as path from 'path';
import { workspace, ExtensionContext, ExtensionMode, window } from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient;

export function activate(context: ExtensionContext) {
  let command = 'caja'; // Release mode default (assumes 'caja' is in $PATH)

  if (context.extensionMode === ExtensionMode.Development) {
    // If running via F5 (Extension Development Host), look for the binary in the /bin folder at the project root
    command = path.join(context.extensionPath, '..', '..', '..', 'bin', 'caja');
    window.showInformationMessage('Caja LSP path: ' + command);
  }

  const serverOptions: ServerOptions = {
    command: command,
    args: ['lsp'],
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'caja' }]
  };

  client = new LanguageClient(
    'cajaLanguageServer',
    'Caja Language Server',
    serverOptions,
    clientOptions
  );

  client.start();
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
