import {ensureBrowser} from '@remotion/renderer';

const status = await ensureBrowser({logLevel: 'warn'});

if (status.type === 'no-browser' || status.type === 'version-mismatch') {
  throw new Error(`Unable to prepare Remotion browser, status: ${status.type}`);
}

console.log(`browser: ${status.type}`);
