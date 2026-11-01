# Mealplan

[![ci](https://github.com/MarcinCho/mealplan/actions/workflows/ci.yml/badge.svg)](https://github.com/MarcinCho/mealplan/actions/workflows/ci.yml)
[![Licencja: MIT](https://img.shields.io/badge/licencja-MIT-blue.svg)](LICENSE)

Lekka domowa aplikacja do planowania jadłospisu na tydzień: kto gotuje które danie, lista zakupów, przepisy. Działa w przeglądarce na telefonie i komputerze, na własnym serwerze (selfhost), z bazą SQLite w jednym pliku.

> Status: Etap 2 (rdzeń): przepisy, tygodniowy plan, lista zakupów oraz konta, zaproszenia i role. Import przepisów, inwentarz, statystyki i tryb hosted są dopiero planowane.

## Funkcje

- Przepisy z listą składników i porcjami.
- Tygodniowy plan: kto gotuje które danie i na ile porcji.
- Lista zakupów generowana z planu.
- Konta domowników, zaproszenia (link ważny 72 godziny) i role.

## Szybki start

Pełny przewodnik krok po kroku (Docker, Docker Compose, kod źródłowy, HTTPS przez Tailscale, kopie zapasowe): [Szybki start](docs/quickstart.md).

```bash
docker build -t mealplan .
docker run -d --name mealplan --restart unless-stopped -p 127.0.0.1:8080:8080 -v mealplan-data:/data -e MEALPLAN_COOKIE_SECURE=false mealplan
# otwórz http://localhost:8080 i utwórz konto administratora
```

`MEALPLAN_COOKIE_SECURE=false` służy tylko do testu po zwykłym HTTP. Na co dzień użyj HTTPS i usuń tę zmienną.

## Dokumentacja

- [Szybki start (selfhost)](docs/quickstart.md)
- [Architektura](docs/architecture.md)
- [Bezpieczeństwo](docs/security.md)
- [Tryby i konfiguracja](docs/modes.md)
- [Wdrożenie selfhost z Tailscale](docs/deployment-selfhost.md)
- [API](docs/api.md)

## Projekt

- [Współpraca](CONTRIBUTING.md)
- [Kodeks postępowania](CODE_OF_CONDUCT.md)
- [Polityka bezpieczeństwa](SECURITY.md)

## Licencja

MIT, zobacz [LICENSE](LICENSE).
