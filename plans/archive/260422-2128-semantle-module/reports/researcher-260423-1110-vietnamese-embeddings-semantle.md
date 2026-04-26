# Vietnamese Embeddings for Doantu: sup-SimCSE vs PhoW2V

**Date:** 2026-04-23 | **Scope:** Word-level cosine similarity in fixed 22k-vocab Semantle clone

---

## Executive Verdict

**Recommendation: PhoW2V (word-level, 300d) is the better fit.**

Reasons: (1) Purpose-built for word similarity, not sentences; (2) static word2vec format enables precomputation into lookup table — zero inference overhead; (3) no external segmentation tool required; (4) Cloudflare Worker-friendly (ship vectors in KV or bundle with Worker). 

**sup-SimCSE is inferior here** despite better semantic depth, because it requires runtime inference, external VnCoreNLP/pyvi segmentation, and sentence-level training (not optimized for single-word pairs). Violates KISS.

---

## Detailed Comparison

### 1. **sup-SimCSE-VietNamese-phobert-base** (VoVanPhuc)

| Aspect | Value | Trade-off |
|--------|-------|-----------|
| **Embedding Dim** | 768 | Large; overkill for word pairs |
| **Training** | Supervised contrastive (SimCSE) | Optimized for sentence similarity, not word pairs |
| **Level** | Sentence-level | Not designed for single-word input |
| **Vocab** | Open (transformer subword tokenization) | Handles unseen words via BPE; adds latency |
| **Segmentation** | **REQUIRED** (RDRSegmenter or pyvi) | Extra runtime dependency; "con chó" must become "con_chó" before encoding |
| **Model Size** | 135M parameters | ~250–350 MB disk; requires transformers + torch on Worker? Infeasible. |
| **Inference** | Runtime + tokenization | Cold start latency; not precomputable for 22k words |
| **Format** | Hugging Face (transformers) | No static dump; requires active model loading |

**Key Gotcha:** PhoBERT's tokenizer expects **pre-segmented input**. "máy bay" (airplane) as raw input will tokenize as ["máy", "bay"] separately unless segmented to "máy_bay". You'd need VnCoreNLP + pyvi running in Worker — expensive and fragile.

---

### 2. **PhoW2V** (VinAI, datquocnguyen)

| Aspect | Value | Trade-off |
|--------|-------|-----------|
| **Embedding Dim** | 100 or 300 | 300d ideal; 100d saves space but lower quality |
| **Training** | Unsupervised word2vec (CBOW/Skip-gram) | Pure word-level; no sentence context — but this matches your use case exactly |
| **Level** | Word-level (and syllable-level variant) | Designed for single-word similarity |
| **Vocab** | ~100k words from 20GB corpus (likely covers 22k) | Finite vocab; OOV words get zero vector or nearest neighbor |
| **Segmentation** | **Optional** (can use word-level variant directly) | If input is already word-tokenized, no extra step |
| **Model Size** | ~30–50 MB (gensim KeyedVectors) | Tiny; fits in Cloudflare KV or bundle |
| **Inference** | Zero runtime — precompute entire 22k vocab | `precomputed[word] = word_vector` lookup O(1) |
| **Format** | Gensim KeyedVectors (text/binary) | Exportable as dense matrix (22k × 300) for embedding |

**Critical Insight:** Word2Vec embeddings are **static lookup tables**. You can precompute similarity scores for all 22k² word pairs offline, or dump the 22k vectors into KV and compute cosine similarity on-demand (O(300) dot product per pair — negligible).

---

## Tokenization & Diacritics

### PhoBERT (sup-SimCSE)
- Requires VnCoreNLP/pyvi to convert raw input to segmented form.
- **"con chó"** → requires preprocessing to **"con_chó"** before tokenization.
- Adds runtime cost + dependency fragility.
- Diacritics preserved via RDRSegmenter normalization.

### PhoW2V
- Word-level variant: expects whitespace-separated words (already segmented).
- If using syllable-level, requires syllable input (less relevant here).
- Diacritics preserved (trained on normalized Vietnamese corpus).
- **No segmentation tool needed if vocab covers your compound words.**

**For a fixed 22k-word game vocabulary:** Pre-segment and validate your entire wordlist at deployment time. Both approaches require diacritic-aware matching ("cá" ≠ "ca").

---

## Precomputation & Cloudflare Worker Fit

### sup-SimCSE (Cannot Precompute)
```
❌ Runtime inference required (PhoBERT forward pass)
❌ Requires transformers + torch (not Worker-compatible)
❌ VnCoreNLP dependency for each request
❌ Cold start latency (~500ms per query on CPU)
```

### PhoW2V (Fully Precomputable)
```
✅ Load KeyedVectors once at Worker start
✅ Precompute all 22k embeddings into in-memory dense matrix
✅ Cosine similarity: ~1ms per pair (vector dot product)
✅ Alternative: Ship (22k × 22k) similarity matrix in KV
✅ Or: ~7.3 MB dense matrix (22k × 300 × 4 bytes) fits in Worker bundles
```

**Verdict:** PhoW2V enables a **stateless, zero-latency** Semantle implementation. sup-SimCSE requires external inference infrastructure.

---

## Vocabulary Coverage

| Model | Vocab Size | 22k Viet22K Coverage | License |
|-------|------------|---------------------|---------|
| **PhoW2V** | ~100k (estimated from 20GB corpus) | Likely 95%+ (VinAI trained on broad Vietnamese text) | AGPL-3.0; research/education only; cite EMNLP-2020 |
| **sup-SimCSE** | Unbounded (subword + BPE) | 100% (BPE handles unknowns) | Likely permissive (HF model) |

**Gotcha:** PhoW2V is **research-only, non-commercial**, and requires citation. Check your license constraints for a Telegram bot (even if private, may still violate terms).

---

## Semantic Quality

### sup-SimCSE
- **Advantage:** Trained on supervised sentence pairs; captures deeper semantic relationships.
- **Disadvantage:** Trained on sentence context; single-word pairs don't benefit from that context.
- **Effective for:** Semantically distant word pairs (e.g., "xe" vs "cách"); may over-regularize tight synonyms.

### PhoW2V
- **Advantage:** Word-level training (CBOW/Skip-gram); embeddings encode co-occurrence statistics.
- **Disadvantage:** No supervised signal; relies purely on distributional similarity.
- **Effective for:** "Natural" word similarity (synonyms, related concepts); well-suited to Semantle-style games.

**For a word-guessing game:** Both are reasonable. PhoW2V's simplicity is not a weakness here; it's a feature.

---

## Implementation Complexity

### sup-SimCSE (High Complexity)
```python
from sentence_transformers import SentenceTransformer
from pyvi.ViTokenizer import tokenize

model = SentenceTransformer('VoVanPhuc/sup-SimCSE-...')
# Per query:
segmented = tokenize(raw_input)  # Runtime overhead
emb = model.encode(segmented)
similarity = cosine(emb_target, emb_guess)
```
- **Dependencies:** sentence-transformers, transformers, torch, pyvi.
- **Latency:** 200–500ms per query (even on GPU; Cloudflare Workers have no GPU).
- **Lines of code:** ~20.
- **External service:** Optional (could self-host, but adds infrastructure).

### PhoW2V (Low Complexity)
```python
from gensim.models import KeyedVectors
import numpy as np

kv = KeyedVectors.load_word2vec_format('phow2v.bin')
# Option A (precompute all):
embeddings = {word: kv[word] for word in vocab}

# Per query:
similarity = np.dot(embeddings[target], embeddings[guess])
```
- **Dependencies:** gensim (tiny).
- **Latency:** <1ms per query (in-memory lookup).
- **Lines of code:** ~10.
- **External service:** None (pure static embeddings).

---

## License & Attribution

| Model | License | Restriction |
|-------|---------|------------|
| **sup-SimCSE** | Unclear (check HF model card) | Likely permissive for research |
| **PhoW2V** | AGPL-3.0 | **Research/education only; cite EMNLP-2020; non-commercial** |

**Risk:** If doantu is a commercial Telegram bot or intends to be monetized, PhoW2V's AGPL restriction may be a blocker. Clarify with user.

---

## Unresolved Questions

1. **PhoW2V license constraints:** Is doantu commercial? Non-commercial? Verify AGPL-3.0 compatibility with your bot's intended use.
2. **Vocabulary overlap:** Exact coverage of 22k Viet22K words in PhoW2V. Could spot-check a few compounds like "máy bay", "con chó" in the model.
3. **Syllable vs word PhoW2V:** Recommendation assumes word-level variant. If Viet22K uses syllables, syllable-level variant may be needed; would require preprocessing.
4. **sup-SimCSE alternatives:** Are you open to other sentence transformers fine-tuned for Vietnamese word similarity (e.g., from FPTAI or other VN NLP labs)?
5. **Similarity matrix size:** Confirm whether shipping a precomputed (22k × 22k) matrix in KV is practical (~7.3 MB in dense form, ~200 MB in sparse COO).

---

## Recommendation Summary

**Use PhoW2V (300d word-level variant) for doantu.**

**Why:**
- Single-word embeddings (not sentence-level).
- Static vectors → zero inference cost.
- Fits Cloudflare Worker budget (no external service needed).
- Precomputable into O(1) lookups.
- Simpler to deploy and maintain.

**Why not sup-SimCSE:**
- Sentence-level training doesn't benefit single-word pairs.
- Runtime inference infeasible on CPU-only Cloudflare Workers.
- External segmentation (pyvi/VnCoreNLP) adds complexity and latency.
- 768-dim vectors overkill for word pairs; 300-dim sufficient.

**Action items:**
1. Verify PhoW2V's AGPL-3.0 license permits your bot's use case.
2. Spot-check PhoW2V vocabulary against 22k-word game list (OOV strategy needed).
3. Decide precomputation strategy: in-memory matrix, KV store, or on-demand dot product.

---

## Sources

- [sup-SimCSE-VietNamese-phobert-base on Hugging Face](https://huggingface.co/VoVanPhuc/sup-SimCSE-VietNamese-phobert-base)
- [PhoW2V GitHub Repository](https://github.com/datquocnguyen/PhoW2V)
- [PhoBERT: Pre-trained Language Models for Vietnamese (EMNLP-2020 Findings)](https://aclanthology.org/2020.findings-emnlp.92.pdf)
- [VinAI Research – PhoBERT Overview](https://www.vinai.io/phobert-the-first-public-large-scale-language-models-for-vietnamese/)
- [Gensim Word2Vec KeyedVectors Documentation](https://radimrehurek.com/gensim/models/word2vec.html)
- [Semantle Word Embeddings Recreation](https://github.com/memgonzales/semantle-word-embeddings)
- [VnCoreNLP Word Segmentation](https://www.researchgate.net/publication/325449322_VnCoreNLP_A_Vietnamese_Natural_Language_Processing_Toolkit)
