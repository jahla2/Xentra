from __future__ import annotations

import hashlib
import math
import os
import re
from dataclasses import dataclass
from typing import Protocol

import httpx


EMBEDDING_DIMENSIONS = 384
_TOKEN_PATTERN = re.compile(r"[a-z0-9_./:-]+")


class EmbeddingError(RuntimeError):
    pass


class EmbeddingProvider(Protocol):
    name: str

    def embed(self, texts: list[str]) -> list[list[float]]:
        ...


class HashEmbeddingProvider:
    name = "hash"

    def __init__(self, dimensions: int = EMBEDDING_DIMENSIONS) -> None:
        self.dimensions = dimensions

    def embed(self, texts: list[str]) -> list[list[float]]:
        return [self._embed_one(text) for text in texts]

    def _embed_one(self, text: str) -> list[float]:
        vector = [0.0] * self.dimensions
        tokens = _TOKEN_PATTERN.findall(text.lower())
        for token in tokens:
            digest = hashlib.blake2b(token.encode("utf-8"), digest_size=8).digest()
            index = int.from_bytes(digest[:4], "big") % self.dimensions
            sign = 1.0 if digest[4] & 1 else -1.0
            vector[index] += sign
        norm = math.sqrt(sum(value * value for value in vector))
        if norm:
            vector = [value / norm for value in vector]
        return vector


@dataclass(frozen=True)
class EmbeddingConfig:
    base_url: str
    api_key: str
    model: str
    dimensions: int = EMBEDDING_DIMENSIONS
    timeout_seconds: float = 20.0


class OpenAICompatibleEmbeddingProvider:
    name = "openai-compatible"

    def __init__(self, config: EmbeddingConfig, client: httpx.Client | None = None) -> None:
        self.config = config
        self.client = client or httpx.Client(timeout=config.timeout_seconds)

    def embed(self, texts: list[str]) -> list[list[float]]:
        if not texts:
            return []
        try:
            response = self.client.post(
                self.config.base_url.rstrip("/") + "/embeddings",
                headers={
                    "Authorization": f"Bearer {self.config.api_key}",
                    "Content-Type": "application/json",
                },
                json={
                    "model": self.config.model,
                    "input": texts,
                    "dimensions": self.config.dimensions,
                },
            )
            response.raise_for_status()
            payload = response.json()
            items = sorted(payload["data"], key=lambda item: item["index"])
            vectors = [item["embedding"] for item in items]
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as exc:
            raise EmbeddingError(f"embedding provider failed: {exc}") from exc

        if len(vectors) != len(texts):
            raise EmbeddingError("embedding provider returned the wrong number of vectors")
        if any(len(vector) != self.config.dimensions for vector in vectors):
            raise EmbeddingError(
                f"embedding provider must return {self.config.dimensions}-dimensional vectors"
            )
        return [[float(value) for value in vector] for vector in vectors]


class EmbeddingEngine:
    def __init__(
        self,
        provider: EmbeddingProvider,
        fallback: EmbeddingProvider | None = None,
    ) -> None:
        self.provider = provider
        self.fallback = fallback

    @property
    def provider_name(self) -> str:
        return self.provider.name

    def embed(self, texts: list[str]) -> list[list[float]]:
        try:
            return self.provider.embed(texts)
        except EmbeddingError:
            if self.fallback is None:
                raise
            return self.fallback.embed(texts)


def build_embedding_engine_from_env() -> EmbeddingEngine:
    api_key = os.getenv("XENTRA_LLM_API_KEY", "").strip()
    model = os.getenv("XENTRA_EMBEDDING_MODEL", "").strip()
    fallback = HashEmbeddingProvider()

    if not api_key or not model:
        return EmbeddingEngine(provider=fallback)

    config = EmbeddingConfig(
        base_url=os.getenv("XENTRA_LLM_BASE_URL", "https://api.openai.com/v1"),
        api_key=api_key,
        model=model,
        dimensions=EMBEDDING_DIMENSIONS,
        timeout_seconds=float(os.getenv("XENTRA_LLM_TIMEOUT_SECONDS", "20")),
    )
    return EmbeddingEngine(
        provider=OpenAICompatibleEmbeddingProvider(config),
        fallback=fallback,
    )
