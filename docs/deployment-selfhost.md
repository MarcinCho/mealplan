# Wdrożenie selfhost z Tailscale

```mermaid
flowchart LR
    T[Telefon lub komputer w tailnecie] -- HTTPS --> TS[tailscale serve]
    TS -- HTTP 127.0.0.1:8080 --> M[Mealplan]
    M --> V[(Wolumen /data z SQLite)]
```

## Docker

```bash
docker build -t mealplan .
docker run -d --name mealplan --restart unless-stopped \
  -p 127.0.0.1:8080:8080 -v mealplan-data:/data mealplan
```

Obraz działa jako użytkownik `nonroot` (uid 65532), a katalog `/data` jest w nim utworzony z tym właścicielem. Wolumen nazwany (`mealplan-data`) działa od razu. Jeśli zamiast wolumenu zamontujesz katalog z hosta (`-v "$PWD/data:/data"`), musi on być zapisywalny dla uid 65532:

```bash
mkdir -p data && sudo chown 65532:65532 data
```

## HTTPS przez Tailscale

```bash
tailscale serve --bg 8080
```

Aplikacja będzie dostępna pod adresem maszyny w tailnecie z certyfikatem HTTPS. Zostaw `MEALPLAN_COOKIE_SECURE=true`.

## Pierwsze uruchomienie

Otwórz adres aplikacji. Pierwsza osoba tworzy gospodarstwo i konto administratora, a kolejne dołączają przez linki zaproszeń (Domownicy, Zaproszenia).

## Zapraszanie osób

- Telefon lub komputer zaproszonej osoby musi być w tym samym tailnecie: ma mieć zainstalowaną aplikację Tailscale i być zalogowana, albo maszyna z Mealplan musi zostać jej udostępniona.
- Zaproszenie utwórz, przeglądając aplikację przez adres HTTPS z tailnetu. Link jest budowany z adresu strony, więc link utworzony na `http://localhost:8080` wskazuje na localhost na urządzeniu odbiorcy i nie zadziała.
- `tailscale serve` z HTTPS wymaga włączonych certyfikatów HTTPS w konsoli administracyjnej Tailscale.
- **Uwaga:** formularz pierwszego uruchomienia może wypełnić każdy, kto ma dostęp do adresu, dopóki właściciel nie zakończy konfiguracji. Wykonaj pierwsze uruchomienie zaraz przed udostępnieniem dostępu i nigdy nie wystawiaj portu publicznie (np. `tailscale funnel` albo mapowanie portu na 0.0.0.0) przed jego zakończeniem.

## Kopia zapasowa

Zatrzymaj kontener i skopiuj plik `mealplan.db` (oraz `-wal` i `-shm`, jeśli istnieją) z wolumenu `/data`. Dla wolumenu nazwanego:

```sh
docker stop <kontener>
docker run --rm -v mealplan-data:/data -v "$PWD":/backup busybox sh -c 'cp /data/mealplan.db* /backup/'
docker start <kontener>
```

Wzorzec `mealplan.db*` obejmuje też pliki `-wal` i `-shm`, jeśli istnieją.
