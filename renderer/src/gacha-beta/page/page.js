// Browser page for the beta wish: one pack-cards pack whose single card shows
// the rolled label, rank, and stars. The renderer injects `window.gachaWish`
// before this module runs, tears the pack open with a scripted drag, and
// captures each frame.
import {createPackView} from 'pack-cards';

/**
 * @typedef {object} GachaWishPage
 * @property {string} label
 * @property {string} rank       Rank letter, as on /api/gacha.
 * @property {number} stars
 * @property {string} rarity     pack-cards rarity profile.
 * @property {'silver' | 'platinum' | 'gold'} glow  pack-cards glow preset.
 * @property {{accent: string, tint: string}} artwork
 */

const wish = /** @type {GachaWishPage} */ (/** @type {any} */ (window).gachaWish);
const stage = /** @type {HTMLElement} */ (document.getElementById('stage'));
stage.style.setProperty('--wish-accent', wish.artwork.accent);
stage.style.setProperty('--wish-tint', wish.artwork.tint);

/**
 * @param {string} tag
 * @param {string} className
 * @param {string} [text]
 */
const el = (tag, className, text) => {
  const node = document.createElement(tag);
  node.className = className;
  if (text !== undefined) {
    node.textContent = text;
  }
  return node;
};

const createFace = () => {
  const face = el('article', 'wish-card');
  const header = el('header', 'recap-card-header');
  header.append(el('span', '', 'GACHA'), el('span', '', `RANK ${wish.rank}`));
  const content = el('div', 'recap-card-content');
  const title = el('h2', 'wish-label', wish.label);
  title.setAttribute('data-foil-text', '');
  content.append(el('span', 'wish-rank', wish.rank), title, el('p', 'wish-stars', '★'.repeat(wish.stars)));
  const footer = el('footer', 'recap-card-footer');
  footer.append(el('span', '', `${wish.stars}★`), el('span', '', '01 / 01'));
  face.append(header, content, footer);
  return face;
};

/**
 * pack-cards reveals the card with a half turn, animating rotateY from 180deg
 * to 0deg. Starting that one animation further round spins the card more
 * before it lands face up, without changing the library.
 */
const flipStartDegrees = 540;
const animate = Element.prototype.animate;
/**
 * @this {Element}
 * @param {Keyframe[] | PropertyIndexedKeyframes | null} keyframes
 * @param {number | KeyframeAnimationOptions} [options]
 */
Element.prototype.animate = function (keyframes, options) {
  const isFlip =
    Array.isArray(keyframes) &&
    keyframes.length === 2 &&
    keyframes[0]?.transform === 'rotateY(180deg)' &&
    keyframes[1]?.transform === 'rotateY(0deg)';
  return animate.call(
    this,
    isFlip ? [{transform: `rotateY(${flipStartDegrees}deg)`}, keyframes[1] ?? {}] : keyframes,
    options,
  );
};

const view = createPackView(stage, {
  label: 'Gacha',
  artwork: wish.artwork,
  labels: {title: 'Gacha', swipe: '', tap: ''},
  appearance: {opening: 'animated', motion: 'interactive', glow: wish.glow, pack_zoom: true},
});

view.showPack({
  count: 1,
  renderCard: () => ({face: createFace(), identity: wish.label, rarity: wish.rarity}),
  onReveal: () => {
    document.documentElement.dataset.wish = 'revealed';
  },
  onError: (error) => {
    document.documentElement.dataset.wish = 'failed';
    console.error(error);
  },
});
