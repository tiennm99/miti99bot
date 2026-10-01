import path from 'node:path';
import {describe, expect, test} from 'vitest';
import {
  gachaBetaTotalSeconds,
  getDragPointer,
  getWishPageProps,
  resolveWishAsset,
  wishOrigin,
  wishTimeline,
} from '../src/gacha-beta/wish-plan.js';

const roots = {page: path.resolve('/srv/page'), packCards: path.resolve('/srv/pack-cards')};

describe('gacha beta wish plan', () => {
  test('the page shows the request label with the same rank and stars as /api/gacha', () => {
    const base = {label: '<b>Bún bò</b>', fps: /** @type {const} */ (24), width: /** @type {const} */ (640), seed: 1};
    expect(getWishPageProps({...base, rarity: 3})).toMatchObject({label: '<b>Bún bò</b>', rank: 'B', stars: 3, rarity: 'rare'});
    expect(getWishPageProps({...base, rarity: 4})).toMatchObject({rank: 'A', stars: 4, rarity: 'epic'});
    expect(getWishPageProps({...base, rarity: 5})).toMatchObject({rank: 'S', stars: 5, rarity: 'legendary'});
    expect(getWishPageProps({...base, rarity: 5}).artwork).toEqual({accent: '#ffd36b', tint: '#7a4a10'});
  });

  test('the drag sweeps left to right across the seal, then lets go', () => {
    const {dragStart, dragEnd} = wishTimeline;
    expect(getDragPointer(dragStart - 0.01)).toBeNull();
    expect(getDragPointer(dragEnd + 0.01)).toBeNull();
    const start = getDragPointer(dragStart);
    const end = getDragPointer(dragEnd);
    expect(start && end).toBeTruthy();
    if (start && end) {
      expect(end.x).toBeGreaterThan(start.x);
      expect(end.x).toBeLessThanOrEqual(1);
      expect(end.y).toBe(start.y);
    }
    expect(dragEnd + 2).toBeLessThan(gachaBetaTotalSeconds);
  });

  test('page requests map to the page directory and the installed package', () => {
    expect(resolveWishAsset(`${wishOrigin}/index.html`, roots)).toEqual({
      file: path.join(roots.page, 'index.html'),
      contentType: 'text/html; charset=utf-8',
    });
    expect(resolveWishAsset(`${wishOrigin}/pack-cards/presentation/pack.js`, roots)).toEqual({
      file: path.join(roots.packCards, 'presentation', 'pack.js'),
      contentType: 'text/javascript; charset=utf-8',
    });
  });

  test('requests outside the page, other origins, and unknown file types are refused', () => {
    for (const url of [
      `${wishOrigin}/..%2fsecret.js`,
      `${wishOrigin}/%2e%2e%2fsecret.js`,
      `${wishOrigin}/pack-cards/..%2f..%2fpage%2fpage.js`,
      `${wishOrigin}/page.sh`,
      `${wishOrigin}/%E0%A4%A`,
      'https://example.com/index.html',
      'not a url',
    ]) {
      expect(resolveWishAsset(url, roots), url).toBeNull();
    }
  });

  test('dot segments are resolved by the URL before mapping, so they stay inside a root', () => {
    const asset = resolveWishAsset(`${wishOrigin}/pack-cards/%2e%2e/%2e%2e/etc/passwd.js`, roots);
    expect(asset?.file).toBe(path.join(roots.page, 'etc', 'passwd.js'));
  });
});
