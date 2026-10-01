---
title: Open-source web gacha projects as a code source for /api/gacha
date: 2026-10-01 16:56 (Asia/Saigon)
---

# Open-source web gacha projects as a code source for /api/gacha

## Answer

No candidate offers reusable code that would make the gacha wish look more like the
real game. The realistic simulators get their look by playing MP4/WebM clips
recorded from the commercial games. Their code is often MIT-licensed, but the
clips belong to the publisher (HoYoverse, Nexon, and others). Mantan21's README
says so directly: "all assets used for this application belongs to Hoyoverse".
The projects with their own art (original games) have simple pixel or HTML
reveals that are weaker than the current gacha wish. Copying either kind of code
would give us one of two things: a `<video>` player that needs copyrighted
footage, or an animation worse than what we already render.

## Candidates

Stars, licences, and dates come from the GitHub API on 2026-10-01.

| # | Project | Stars | Stack | Code licence | How the pull animation is made | Reusable for us? |
|---|---|---|---|---|---|---|
| 1 | [Mantan21/Genshin-Impact-Wish-Simulator](https://github.com/Mantan21/Genshin-Impact-Wish-Simulator) ([live](https://wishsimulator.app)) | 293 | Svelte | MIT | Plays game-recorded clips (`3star-single.mp4` … `5star-multi.mp4`, `bg.webm`) through `_meteor.svelte` | Code yes, footage no |
| 2 | [shadorki/genshin-impact-wish-simulator](https://github.com/shadorki/genshin-impact-wish-simulator) | 734 | JavaScript | None (all rights reserved) | Plays `dist/videos/5starwish.mp4` and similar game clips | No |
| 3 | [Mantan21/HSR-Warp-Simulator](https://github.com/Mantan21/HSR-Warp-Simulator) ([live](https://hsr.wishsimulator.app)) | 40 | Svelte | MIT | Same author and approach, built on Star Rail game assets; the warp clip source was not checked | Code yes, assets no |
| 4 | [jianzhishendi/Genshin-Impact-Wish-Simulator-1](https://github.com/jianzhishendi/Genshin-Impact-Wish-Simulator-1) | — | Svelte | Fork of #1 | Same as #1 | Same as #1 |
| 5 | [U1805/ba-gacha](https://github.com/U1805/ba-gacha) | 18 | Vue | MIT | `VideoView.vue` plays Blue Archive clips per star tier | Code yes, footage no |
| 6 | [dzmuh97/genshin-twitch-wish](https://github.com/dzmuh97/genshin-twitch-wish) | 2 | Python | None | Composites game `effect.mp4` backgrounds | No |
| 7 | [Eling486/gacha-simulator](https://github.com/Eling486/gacha-simulator) | 23 | JavaScript | None | Arknights Spine skeletons (`.atlas`/`.skel`) extracted from the game | No |
| 8 | [Spelljinxer/nikke-gacha-sim](https://github.com/Spelljinxer/nikke-gacha-sim) | 9 | HTML | None | Game images, no video | No |
| 9 | [Catgenova/browsergacha](https://github.com/Catgenova/browsergacha) | 0 | Vanilla JS + canvas | None | Original pixel-art summon screen with free UI packs | No licence, and simpler than ours |
| 10 | [HauntedBees/Public-Domains](https://github.com/HauntedBees/Public-Domains) | 1 | JavaScript | AGPL-3.0 | Public-domain art, static reveal; last push 2019 | AGPL would apply to our service; little animation to gain |
| 11 | [DragonMarquise/GachaGameSimulator](https://github.com/DragonMarquise/GachaGameSimulator) | 7 | HTML/CSS/JS | AGPL-3.0 | Basic CSS reveal | AGPL, and simpler than ours |

Tracker and analyser tools (for example `biuuu/genshin-wish-export`,
`lgou2w/HoYo.Gacha`, `bhaoo/endfield-gacha`) came up in the searches but have
no pull animation, so they are excluded.

## What the code would actually give us

- **Video player and skip handling (#1, #5).** We already render to MP4, so
  there is nothing to gain.
- **Pull rates and pity logic (#1).** MIT and realistic. It doesn't fit our
  contract, though: the bot picks the winner uniformly and the caller sets
  `rarity`. Adopting it would change bot behaviour, not visuals.
- **Spine or Lottie playback (#7).** The technique could work through
  `@remotion/lottie`, but only with original animation files, and none exist
  in these repositories.

## Honest options from here

1. **Keep procedural rendering and add original painted textures**, as
   recommended in the previous discussion: images generated once with an
   image model or supplied by you, animated by the current code.
2. **Commission or author a Lottie or Spine wish animation** and play it
   through `@remotion/lottie`. This gives the highest fidelity without game
   assets, but someone must make the animation file.
3. **Use the game's recorded clips** as #1–#6 do. Not recommended: HoYoverse
   owns the footage, the README promises no game assets, and the bot is public.

## Method

GitHub search (`gh search repos`, `--topic gacha`), repository trees and
READMEs read through the GitHub API, plus two web searches. I checked
animation sources by listing video and Spine files in each tree and reading
Mantan21's `meteor-loader.js` and `_meteor.svelte`.

## Unresolved questions

- Is there a commercial or Creative Commons Lottie wish animation you would
  accept, if we went with option 2?
- How does the HSR simulator (#3) source its warp clip? I didn't check,
  because it would not change the recommendation.
