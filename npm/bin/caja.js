#!/usr/bin/env node

const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const platform = os.platform();
const arch = os.arch();

// Determine the package name for the current platform and architecture
const packageName = `@caja/cli-${platform}-${arch}`;

let binPath;
try {
  // Use require.resolve to find the path to the optional dependency package
  const packagePath = require.resolve(`${packageName}/package.json`);
  const packageDir = path.dirname(packagePath);
  
  // The binary name is 'caja' (or 'caja.exe' on Windows)
  const binName = platform === 'win32' ? 'caja.exe' : 'caja';
  binPath = path.join(packageDir, 'bin', binName);
} catch (error) {
  console.error(`Error: Unsupported platform or architecture: ${platform}-${arch}`);
  console.error(`The package ${packageName} could not be found.`);
  console.error('Make sure you have installed the correct optional dependencies.');
  process.exit(1);
}

// Spawn the binary, passing all arguments from the original command line
const { status, error } = spawnSync(binPath, process.argv.slice(2), { stdio: 'inherit' });

if (error) {
  console.error(`Failed to execute binary: ${error.message}`);
  process.exit(1);
}

process.exit(status);
