# Mealplan

Lekka domowa aplikacja do planowania jadłospisu na tydzień: kto gotuje które danie, lista zakupów, import przepisów, inwentarz i statystyki. Działa w przeglądarce na telefonie i komputerze. Licencja MIT.

> Status: Etap 1 (fundament): konta, logowanie, zaproszenia i powłoka UI. Planowanie, przepisy i zakupy w kolejnych etapach.

## Szybki start

```bash
docker build -t mealplan .
docker run -d -p 127.0.0.1:8080:8080 -v mealplan-data:/data -e MEALPLAN_COOKIE_SECURE=false mealplan
```

Otwórz `http://localhost:8080` i utwórz konto administratora. Do codziennego użytku ustaw HTTPS (np. `tailscale serve`) i zostaw `MEALPLAN_COOKIE_SECURE=true`. Po włączeniu HTTPS kontener trzeba utworzyć ponownie bez zmiennej `MEALPLAN_COOKIE_SECURE=false` (`docker rm -f` i `docker run` bez `-e`).

Samo `go build` bez wcześniejszego `npm run build` daje pusty interfejs. Używaj `make build` albo Dockera.

Wolumen nazwany działa od razu. Przy montowaniu katalogu z hosta (`-v "$PWD/data:/data"`) katalog musi być zapisywalny dla uid 65532 (`sudo chown 65532:65532 ./data`), bo obraz działa jako `nonroot`.

Z kodu źródłowego (Go 1.26, Node 22):

```bash
make run
```

## Dokumentacja

- [Architektura](docs/architecture.md)
- [Bezpieczeństwo](docs/security.md)
- [Tryby i konfiguracja](docs/modes.md)
- [Wdrożenie selfhost z Tailscale](docs/deployment-selfhost.md)
- [API](docs/api.md)
- [Współpraca](CONTRIBUTING.md)

## Licencja

MIT, zobacz [LICENSE](LICENSE).
