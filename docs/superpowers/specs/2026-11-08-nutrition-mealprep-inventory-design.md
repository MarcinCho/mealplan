# Mealplan: wartości odżywcze, inwentarz i meal prep

Data: 2026-11-08 · Status: do przeglądu · Rozszerza `2026-11-01-mealplan-design.md` (§7.4 inwentarz) i `2026-11-03-recipe-parser-design.md`

## 1. Po co

Właściciel przygotowuje się do triathlonu i często je za mało w ciągu dnia. Chce widzieć, ile kalorii, białka, węglowodanów i tłuszczu dostarczają zaplanowane posiłki, i porównywać to z własnym celem. Gospodarstwo (dwie osoby) gotuje wspólnie w niedzielę kilka dań na raz (np. 6, 8, 4 i 12 porcji), rozdziela porcje na posiłki w tygodniu, a resztę mrozi. Lista zakupów ma uwzględniać, co już jest w domu, żeby nie kupować drugi raz.

**Kryteria sukcesu**
- Pod każdym dniem widać sumę kalorii i makro dla jednej osoby oraz ile brakuje do jej celu.
- Po niedzielnym gotowaniu w kilka minut rozdzielasz porcje na tydzień i wiesz, co leży w zamrażalce.
- Lista zakupów nie zawiera rzeczy, które są w domu, a inwentarz da się utrzymać jednym przeglądem w miesiącu.

## 2. Zakres i kolejność

Trzy części, wdrażane w tej kolejności, każda z osobnym planem:

1. **Wartości odżywcze**: baza produktów, makro przepisów, cele i dopisane pozycje na koncie, podsumowanie dnia, dopisanie makro do skilla.
2. **Inwentarz**: stan produktów w domu, przegląd miesięczny, odejmowanie na liście zakupów.
3. **Meal prep**: sesje gotowania, przydział porcji, zamrażalka, posiłek „na mieście”.

Część 1 wprowadza bazę produktów, z której korzystają części 2 i 3.

**Poza zakresem (non-goals)**: skanowanie kodów kreskowych, pobieranie danych z zewnętrznych baz w czasie działania (Open Food Facts itp.), mikroelementy (witaminy, sód, błonnik), śledzenie wagi ciała, wyliczanie celu z treningu lub wzorów (cel wpisuje się ręcznie), różna wielkość porcji dla różnych osób (porcja to porcja; więcej jedzenia dopisuje się osobno), powiadomienia o terminach ważności.

**Czego nie wolno zepsuć (anti-goals)**
- Podsumowanie nigdy nie może po cichu zaniżać: jeśli czegoś nie da się policzyć, jest to widoczne.
- Inwentarz nie może wymagać codziennej obsługi; jeśli rozjedzie się z rzeczywistością, przegląd miesięczny go naprawia.
- Meal prep nie może podwójnie liczyć zakupów (sesja i przydzielone z niej posiłki).

## 3. Ograniczenia

- Istniejący stos: Go, SQLite i Postgres (migracje przenośne na oba silniki), Vue 3 + Vuetify 3 + Pinia, OpenAPI jako kontrakt (`TestRoutesMatchOpenAPI`), sesja + CSRF, gospodarstwo jako tenant, interfejs po polsku, mobile-first.
- Aplikacja działa bez zewnętrznych usług. Dane startowe bazy produktów są dołączone do aplikacji i muszą mieć licencję zgodną z MIT (np. dane z domeny publicznej, takie jak USDA FoodData Central, z polskimi nazwami); nie wolno dołączać danych z tabel chronionych prawem autorskim.
- Tabela `plan_entry` ma dziś warunek „albo przepis, albo wolny tekst” (`CHECK ((recipe_id IS NOT NULL) <> (free_text <> ''))`). Nowe rodzaje wpisów wymagają zmiany tego warunku; w SQLite to przebudowa tabeli w migracji.
- Generator listy zakupów (`internal/shopping/Build`) ma już parametr zapasów (`stock`), który dziś dostaje pustą listę; część 2 go wypełnia.

## 4. Część 1: wartości odżywcze

### 4.1 Baza produktów
- Produkt gospodarstwa: nazwa, kalorie, białko, węglowodany i tłuszcz **na 100 g** (decyzja właściciela), opcjonalna **gramatura** sztuki lub opakowania (np. baton 45 g, jajko 50 g), opcjonalne nazwy alternatywne (np. „mąki”, „mąka pszenna typ 450”) do dopasowania składników.
- Startowy zestaw ok. 150–250 typowych produktów z uśrednionymi wartościami (mąka, jajka, ryż, makaron, kurczak, mleko, warzywa, oleje…), dołączony do aplikacji i kopiowany do gospodarstwa przy pierwszym użyciu, żeby można go było edytować. Wybór źródła i listy należy do budującego, w granicach §3.
- Właściciel dodaje własne produkty (np. konkretny baton) i poprawia istniejące. Baza jest wspólna dla gospodarstwa.

### 4.2 Wartości przepisu
- Każdy przepis ma kalorie, białko, węglowodany i tłuszcz **na jedną porcję** oraz źródło wartości: `z przepisu` (podane przez źródło lub wpisane ręcznie) albo `wyliczone`.
- Kolejność (decyzja właściciela): najpierw wartości z przepisu; jeśli ich nie ma, aplikacja wylicza je ze składników przez bazę produktów.
- Import z linku czyta `nutrition` z JSON-LD (`NutritionInformation`, wartości na porcję). Import z tekstu czyta linię wartości (§4.6).
- Wyliczanie: każdy składnik jest dopasowany do produktu (automatycznie po nazwie znormalizowanej i nazwach alternatywnych; w edytorze można wybrać produkt ręcznie i ten wybór jest zapamiętany). Ilość przeliczana na gramy: `g`/`kg` wprost; `szt`, `ząbek`, `plaster`, `kostka`, `opakowanie`, `puszka`, `kromka`, `główka` przez gramaturę produktu; `ml`/`l`, `łyżka` (15 ml), `łyżeczka` (5 ml), `szklanka` (250 ml) przez objętość (domyślnie 1 g/ml, produkt może mieć własną gęstość, np. olej). Suma dzielona przez liczbę porcji przepisu.
- Składnik bez dopasowania, bez ilości lub bez możliwej zamiany na gramy jest pomijany, a wynik jest oznaczony jako **przybliżony** z listą pominiętych składników („sól do smaku” bez ilości liczy się jako 0 i nie psuje oznaczenia).
- Wartości wyliczone aktualizują się, gdy zmieni się przepis albo produkt. Ręcznie wpisane wartości wygrywają, dopóki użytkownik ich nie usunie.

### 4.3 Konto: śledzenie i cel
- W ustawieniach konta (nowy ekran „Konto”, dostępny z menu awatara) można włączyć śledzenie wartości odżywczych i wpisać dzienny cel: kalorie, białko, węglowodany, tłuszcz (każde opcjonalne). Domyślnie wyłączone. Każdy domownik ma własne ustawienia (decyzja właściciela).

### 4.4 Dopisane pozycje
- Konto ze śledzeniem może dopisać do dnia własną pozycję: **produkt z bazy i ilość** (w gramach albo w sztukach, jeśli produkt ma gramaturę). Pozycja liczy się tylko w podsumowaniu tego konta. Z tego samego miejsca można dodać nowy produkt do bazy, jeśli go nie ma.

### 4.5 Podsumowanie dnia
- Na ekranie „Dziś” pod listą posiłków i w widoku tygodnia pod każdym dniem (tylko dla kont ze śledzeniem): suma **jednej porcji** każdego zaplanowanego posiłku z przepisem (wszystkie zaplanowane, niezależnie od „Zrobione”; decyzja właściciela) plus dopisane pozycje konta, obok celu i różnicy („brakuje 640 kcal, 38 g białka”).
- Posiłki bez wartości (wolny tekst, „na mieście” bez wpisanych wartości) i przepisy z wartościami przybliżonymi są w sumie wyraźnie oznaczone.
- Kliknięcie w posiłek pokazuje wartości jednej porcji i ich źródło.

### 4.6 Skill i import tekstu
- Format tekstu dostaje opcjonalną linię wartości po linii porcji, którą rozpoznaje parser tekstu i wypisuje skill, gdy źródło podaje wartości:
  `Wartości na porcję: 520 kcal, białko 32 g, węglowodany 60 g, tłuszcz 14 g`
  (dowolna kolejność i podzbiór; angielskie `Per serving:` z `protein`, `carbs`, `fat` też). Skill nie zgaduje wartości; gdy źródło ich nie ma, pomija linię, a aplikacja wyliczy je sama.
- Aktualizacja `web/public/claude-skill/SKILL.md`, `docs/import.md` i testu kanonicznego pliku.

## 5. Część 2: inwentarz

### 5.1 Pozycje
- Pozycja inwentarza: produkt z bazy (tworzony w locie, jeśli go nie ma, bez makro), ilość i jednostka, opcjonalna data ważności, opcjonalna flaga **stały zapas** (przyprawy, olej: nie liczy się ilości i nie trafia na listę zakupów).
- Widok: lista z wyszukiwaniem, edycja w miejscu, szybkie +/−.

### 5.2 Przegląd miesięczny
- Osobny tryb „Przegląd” (decyzja właściciela: raz w miesiącu wspólnie): przechodzi listę pozycja po pozycji, pozwala szybko ustawić ilość albo „nie mamy” (usuwa pozycję), dodać brakujące i zakończyć przegląd. Aplikacja zapamiętuje datę ostatniego przeglądu i po ponad 30 dniach pokazuje przypomnienie na ekranie „Dziś”.

### 5.3 Aktualizacja między przeglądami
- Odhaczone na liście zakupów pozycje można jednym kliknięciem przenieść do inwentarza (dodają się do ilości).
- Oznaczenie posiłku z przepisem jako „Zrobione” oraz oznaczenie sesji meal prepu jako ugotowanej odejmuje składniki (przeskalowane do porcji) z inwentarza, gdy jednostki da się porównać; stan nie spada poniżej zera. Cofnięcie „Zrobione” oddaje odjęte ilości.

### 5.4 Lista zakupów
- Generator odejmuje inwentarz od potrzeb przez wspólny produkt (lub nazwę znormalizowaną), tylko przy zgodnych jednostkach (g/kg, ml/l, szt); pozycje stałego zapasu nie trafiają na listę.
- Pozycja w całości pokryta przez inwentarz nie znika bez śladu: lista pokazuje ją jako „masz w domu: 500 g” (nie do kupienia).

## 6. Część 3: meal prep

### 6.1 Sesja gotowania (układ A, wybrany przez właściciela)
- Zakładka „Meal prep”: sesja ma datę i nazwę („Gotowanie 2 XI”) oraz dania: przepis i liczbę porcji.
- Karta dania pokazuje pasek: porcje w planie, w zamrażalce, pozostałe, oraz przydziały („Pn śniadanie ×2”).
- Przydział tworzy wpis w planie tygodnia oznaczony „z meal prepu” i zabiera domyślnie tyle porcji, ile wynosi ustawienie gospodarstwa „porcji na posiłek” (domyślnie 2); przy konkretnym przydziale można to zmienić.
- Wpisy z meal prepu widać na ekranie „Dziś” i w tygodniu jak inne posiłki i liczą się w podsumowaniu wartości (jedna porcja na osobę).

### 6.2 Zamrażalka
- Pozostałe porcje dania można oznaczyć „❄ zamroź”. Zamrażalka jest jedna dla gospodarstwa i obejmuje wszystkie sesje: danie, liczba porcji, data zamrożenia. Można też ręcznie dodać porcje (np. resztki).
- Z zamrażalki przydziela się porcje do posiłków tak samo jak świeże, także w kolejnych tygodniach. Usunięcie przydziału oddaje porcje do miejsca, z którego pochodziły.

### 6.3 Zakupy i pozostałe posiłki
- Gospodarstwo robi zakupy w sobotę na niedzielne gotowanie i cały następny tydzień, a plan następnego tygodnia jest wtedy już gotowy (decyzja właściciela). Dlatego sesja należy do **tygodnia, na który się gotuje**: sesja w sobotę lub niedzielę domyślnie do następnego tygodnia, w inne dni do bieżącego; przy sesji można to zmienić. Lista zakupów tego tygodnia zawiera składniki na **wszystkie** porcje dań sesji, także gdy sesja wypada przed poniedziałkiem lub poza wybranym zakresem dni. Wpisy w planie przydzielone z meal prepu lub zamrażalki nie dodają składników drugi raz.
- Wolne posiłki uzupełnia się jak dotąd (przepis z kucharzem lub wolny tekst).
- Nowy rodzaj wpisu: **„na mieście”** z opcjonalnym opisem i opcjonalnymi ręcznymi wartościami odżywczymi na osobę; bez kucharza i bez zakupów.

## 7. Rozstrzygnięcia przyjęte przy pisaniu (decyzje autora dokumentu, nie właściciela)

- **Porcje sesji a przepis**: liczba porcji dania w sesji skaluje składniki (12 porcji przepisu na 4 porcje to trzy razy więcej składników); wielkość i wartości jednej porcji się nie zmieniają.
- **Częściowe wartości**: źródło jest zapisywane osobno dla każdej z czterech wartości. Wartości podane przez przepis lub wpisane ręcznie są używane, brakujące są wyliczane ze składników; wartość, której nie da się ani odczytać, ani wyliczyć, jest pokazywana jako brak, a suma dnia oznaczona jako niepełna.
- **Zakupy a inwentarz w czasie**: generator listy liczy tylko posiłki nieoznaczone jako „Zrobione” (zrobione zostały już odjęte z inwentarza) i odejmuje inwentarz od potrzeb generowanego tygodnia. Zapasy nie są rezerwowane między tygodniami; jeśli w poprzednim tygodniu zostały niezrobione posiłki, lista ostrzega, że część zapasów może być już na nie potrzebna.
- **„Zrobione” przy porcjach z meal prepu lub zamrażalki** niczego nie odejmuje z inwentarza: składniki odjęło już oznaczenie sesji jako ugotowanej.
- **Nieprzydzielone świeże porcje** zostają na karcie dania jako „pozostałe”, dopóki nie zostaną przydzielone, zamrożone albo oznaczone jako zjedzone lub wyrzucone. Gdy sesja ma pozostałe świeże porcje dłużej niż 4 dni, ekran „Dziś” podpowiada, żeby je zamrozić albo rozdysponować.
- **Ustawienia odżywcze i dopisane pozycje** należą do osoby w danym gospodarstwie (członkostwo), bo baza produktów jest per gospodarstwo.
- **Przypomnienie o przeglądzie inwentarza** po 30 dniach; **„porcji na posiłek”** jako ustawienie gospodarstwa z domyślną wartością 2; **ręczne dodawanie porcji do zamrażalki**; **ręczne wartości dla „na mieście”** na osobę.

## 8. Co zostaje budującemu
- Model danych, migracje (w tym przebudowa `plan_entry`), nazwy tras API, podział na pakiety, rozmieszczenie w interfejsie poza tym, co opisano wyżej.
- Wybór i przygotowanie startowej bazy produktów (§4.1) w granicach licencyjnych z §3, w tym polskie nazwy i nazwy alternatywne.
- Algorytm dopasowania składnika do produktu, pod warunkiem że wybór ręczny jest zapamiętywany i ma pierwszeństwo.
- Szczegóły wyświetlania podsumowania na telefonie i komputerze.
- Kolejność zadań w planach i to, jak każda część jest testowana (z wymogiem testów dla obu silników bazy).
