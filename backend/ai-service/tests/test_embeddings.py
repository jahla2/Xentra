import json

import httpx
import pytest

from app.embeddings import (
    EMBEDDING_DIMENSIONS,
    EmbeddingConfig,
    EmbeddingEngine,
    HashEmbeddingProvider,
    OpenAICompatibleEmbeddingProvider,
)


def test_hash_embeddings_are_deterministic_and_normalized() -> None:
    provider = HashEmbeddingProvider()
    first = provider.embed(["database connection refused"])[0]
    second = provider.embed(["database connection refused"])[0]
    assert first == second
    assert len(first) == EMBEDDING_DIMENSIONS
    norm = sum(value * value for value in first) ** 0.5
    assert norm == pytest.approx(1.0)


def test_similar_texts_share_signal() -> None:
    provider = HashEmbeddingProvider()
    a, b, c = provider.embed(
        [
            "postgres database connection refused",
            "database connection to postgres refused",
            "nginx static asset cache",
        ]
    )
    dot_ab = sum(x * y for x, y in zip(a, b, strict=True))
    dot_ac = sum(x * y for x, y in zip(a, c, strict=True))
    assert dot_ab > dot_ac


def test_openai_compatible_embedding_provider_validates_dimensions() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        payload = json.loads(request.content)
        assert payload["dimensions"] == EMBEDDING_DIMENSIONS
        vector = [0.5] * EMBEDDING_DIMENSIONS
        return httpx.Response(
            200,
            json={
                "data": [
                    {"index": 0, "embedding": vector},
                    {"index": 1, "embedding": vector},
                ]
            },
        )

    provider = OpenAICompatibleEmbeddingProvider(
        EmbeddingConfig(
            base_url="https://llm.example/v1",
            api_key="key",
            model="embedding-model",
        ),
        client=httpx.Client(transport=httpx.MockTransport(handler)),
    )
    vectors = provider.embed(["one", "two"])
    assert len(vectors) == 2
    assert len(vectors[0]) == EMBEDDING_DIMENSIONS


def test_embedding_engine_falls_back_to_hash_provider() -> None:
    class FailingProvider:
        name = "failing"

        def embed(self, texts: list[str]) -> list[list[float]]:
            from app.embeddings import EmbeddingError

            raise EmbeddingError("provider unavailable")

    engine = EmbeddingEngine(FailingProvider(), fallback=HashEmbeddingProvider())
    vectors = engine.embed(["fallback works"])
    assert len(vectors) == 1
    assert len(vectors[0]) == EMBEDDING_DIMENSIONS
