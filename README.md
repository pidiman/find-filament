# Find Filament · Go

Slovenská mobilná PWA: **FILAMENT ME → odfotiť → voliteľný názov → HEX**.

Server je napísaný v **Go** so štandardnou knižnicou, bez externých Go závislostí. HTML, CSS, JavaScript, ikony a offline service worker sú zabudované do jedného spustiteľného súboru pomocou `go:embed`. Na cieľovom serveri netreba Node.js, Go ani priečinok `public/`.

## Čo aplikácia vie

- Bez názvu určí dominantnú farbu zo stredu fotky priamo v zariadení, aj offline.
- Ťuknutím na fotku vyberieš konkrétne miesto; na klávesnici fungujú šípky.
- S názvom odošle zmenšenú fotku, vybraný HEX a názov serveru. OpenAI Responses API s webovým vyhľadávaním sa pokúsi nájsť presný filament na stránke výrobcu.
- Výsledok odlišuje výrobcom uvedený HEX od vizuálneho odhadu a obsahuje zdroj. Neoverený výsledok alebo chyba služby ponechá odhad z fotky.
- Kopírovanie HEX, posledných 12 výsledkov v miestnom úložisku, inštalácia na plochu a offline režim. Fotografie sa do histórie neukladajú.

Mobilná PWA ostáva v HTML/CSS/JavaScripte, pretože fotoaparát a offline režim bežia v prehliadači. Go zabezpečuje server, online vyhľadávanie a doručenie zabudovaných súborov.

## Jeden spustiteľný súbor

Pri použití hotového balíka pre svoju architektúru rozbaľ súbory do jedného priečinka, skopíruj `.env.example` na `.env` a spusti:

```sh
chmod +x find-filament
cp .env.example .env
./find-filament
```

Otvor `http://localhost:3000`. Bez API kľúča fungujú všetky miestne funkcie. Pre online vyhľadávanie v `.env` nastav:

```dotenv
PORT=3000
HOST=0.0.0.0
OPENAI_API_KEY=tvoj_kluc
OPENAI_MODEL=gpt-4.1
LOOKUP_PASSWORD=tvoje_heslo
```

`.env` sa automaticky číta z aktuálneho pracovného priečinka. Iný súbor zvolíš cez `./find-filament -env /cesta/filament.env`. Premenné prostredia majú prednosť pred súborom, aj keď sú prázdne. Podporované sú riadky `KEY=value`, prázdne riadky, komentáre na samostatnom riadku, `export KEY=value` a hodnoty v jednoduchých alebo dvojitých úvodzovkách. Hodnoty s medzerami či `#` dávaj do úvodzoviek; premenné sa v hodnotách nerozbaľujú. Po zmene konfigurácie reštartuj proces.

Online vyhľadávanie vyžaduje platný API kľúč, podporovaný model a kredit na OpenAI API; účtuje sa model aj webové vyhľadávanie. Kľúč nedávaj do `public/`, mobilného rozhrania ani Gitu. `LOOKUP_PASSWORD` chráni platené vyhľadávanie: zadáva sa v nastaveniach aplikácie a ostáva iba v pamäti otvorenej stránky.

## Zostavenie zo zdrojov

Na zostavenie potrebuješ Go 1.26 alebo novšie. Odporúčaná a overená verzia je Go 1.27.3.

```sh
go build -trimpath -ldflags="-s -w" -o find-filament .
./find-filament
```

Na vývoj môžeš použiť `go run .`. Pri zmene súborov v `public/` znovu zostav alebo spusti Go proces, aby sa zmeny vložili do aplikácie.

Zostavenie pre bežný Linux server z ľubovoľného podporovaného počítača:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o find-filament .
```

Pre ARM64 server (napríklad Raspberry Pi so 64-bitovým systémom) použi `GOARCH=arm64`. Vyber zostavenie podľa operačného systému a architektúry cieľového servera. Súbor zo zostavenia obsahuje celé rozhranie; pri prenose stačí tento súbor a vlastná konfigurácia.

## Docker a HTTPS

```sh
cp .env.example .env
# Uprav .env a doplň kľúč a heslo.
docker compose up -d --build
```

Docker zostaví Go aplikáciu a spustí ju v minimálnom obraze ako používateľ bez práv správcu. Obsahuje binárku a koreňové certifikáty na HTTPS spojenie s OpenAI. Go ani Node.js nie sú súčasťou výsledného obrazu. Compose načíta `.env` ako premenné prostredia.

Aplikácia je dostupná na `127.0.0.1:3000`. Pred ňu daj HTTPS reverzný proxy server, napríklad Caddy:

```caddyfile
filament.tvoja-domena.sk {
    reverse_proxy 127.0.0.1:3000
}
```

Pre Nginx v HTTPS server bloku:

```nginx
location / {
    proxy_pass http://127.0.0.1:3000;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 100s;
    client_max_body_size 3m;
}
```

Odporúčané nasadenie je v koreňovej ceste `/` na samostatnej doméne alebo subdoméne. Pri priamom spustení binárky zabezpeč jej automatický štart napríklad cez systemd. Proces spracuje SIGINT/SIGTERM a dá rozbehnutým požiadavkám čas na dokončenie. Štandardný timeout online vyhľadávania je 75 sekúnd.

Na mobile použi **HTTPS**. Výnimkou pre vývoj je localhost na tom istom zariadení. LAN IP cez HTTP neposkytuje plnohodnotnú inštalovateľnú PWA. Tlačidlo používa natívny výber fotografie s `capture="environment"`; podporované mobilné prehliadače ponúknu zadný fotoaparát, iné môžu ponúknuť výber súboru. Galéria je dostupná samostatne.

Inštalácia: Android → menu prehliadača → Inštalovať / Pridať na plochu; iOS Safari → Zdieľať → Pridať na plochu. Offline použitie vyžaduje aspoň jedno úspešné otvorenie online. Samotný priečinok `public/` možno hostiť aj staticky pre farbu z fotky a históriu; kompletné online funkcie vyžadujú bežiaci Go server.

## Portainer + Nginx Proxy Manager na inom stroji

V Portaineri vytvor stack `find-filament` zo zdroja `https://github.com/pidiman/find-filament.git`, vetvy `refs/heads/main` a súboru **`compose.portainer.yaml`**. Tento súbor nepotrebuje serverový súbor `.env`: premenné nastavuješ priamo v Portaineri.

Aktuálne nasadenie používa port **8098** na hostiteľovi aj v kontajneri. Nginx Proxy Manager smeruje **`find-filament.pidiman.sk` → `http://192.168.1.27:8098`**, má zapnuté HTTPS a Force SSL. Port 3000 sa pri tomto nasadení nepoužíva.

API kľúč doplníš v **Stacks → find-filament → Edit stack settings → Environment variables**:

- `OPENAI_API_KEY`: vlastný API kľúč.
- `OPENAI_MODEL`: `gpt-4.1`.
- `LOOKUP_PASSWORD`: voliteľné vlastné heslo pre platené vyhľadávanie; zadáva sa aj v nastaveniach mobilnej aplikácie.

Po úprave premenných stack znovu nasaď cez jeho ovládanie v Portaineri, aby sa kontajner vytvoril s novými hodnotami. Kľúč nepatrí do repozitára ani do konfigurácie proxy. Bez kľúča ostáva dostupná farba z fotky a história.

V Nginx Proxy Manageri je vlastná konfigurácia:

```nginx
client_max_body_size 3m;
proxy_read_timeout 100s;
proxy_send_timeout 100s;
```

## Presnosť a súkromie

HEX z fotografie je odhad z najčastejšieho odtieňa v malej oblasti, nie kalibrované meranie. Foť pri rozptýlenom dennom svetle a vyber miesto bez odlesku, tieňa a cievky. Priehľadné, metalické, silk, trblietavé a viacfarebné materiály nemusia mať jeden reprezentatívny HEX. Výrobcom udaný HEX má prednosť len ak ho vyhľadávanie nájde na konkrétnej stránke; model môže napriek kontrolám urobiť chybu, preto je zdroj viditeľný na overenie.

Bez názvu fotka zariadenie neopustí. S názvom a stlačením „Nájsť môj HEX“ sa do OpenAI odošle názov a JPEG zmenšený na maximálne 1200 px spolu s vybraným odtieňom. Server fotografie nezapisuje na disk a požiadavky používajú `store: false`; na spracovanie dát poskytovateľom sa vzťahujú jeho API podmienky. História obsahuje HEX, názov, vysvetlenie a zdroj a dá sa vymazať tlačidlom. Service worker ukladá iba súbory aplikácie; API odpovede necachuje.

Server obmedzuje veľkosť požiadavky, počet vyhľadávaní a paralelné volania. Limit je 6 požiadaviek za minútu na priamo pripojenú IP a 3 súčasné volania, bezpečne aj pri súbežných požiadavkách. Za reverzným proxy môže byť limit zdieľaný všetkými používateľmi; server nedôveruje hlavičke `X-Forwarded-For`. Pri viacerých procesoch je limit samostatný pre každý proces. Pre osobnú verejne dostupnú aplikáciu nastav `LOOKUP_PASSWORD`. Databáza nie je potrebná.

## Testy

```sh
go test -race ./...
go vet ./...
```

Go testy pokrývajú zabudované súbory a ich typy, konfiguráciu, overenie vstupov, zdroj výrobcu, autentifikáciu, pôvod požiadaviek, veľkosť obrázka, súbežné limity, chyby online služby a formát požiadavky so simulovanou odpoveďou. Reálny OpenAI kľúč nepoužívajú.

Voliteľný test algoritmu farby v prehliadači používa Node.js iba ako vývojový testovací nástroj:

```sh
node --test tests/color.test.mjs
```

Node.js nie je potrebný na zostavenie ani prevádzku aplikácie. Živé vyhľadávanie vyžaduje vlastný API kľúč. Otvorenie fotoaparátu a inštaláciu ešte over na svojom mobile v cieľovom prehliadači.

Pri aktualizácii obsahu zvýš verziu `CACHE` v `public/sw.js`. Nový service worker sa aktivuje po zavretí starých okien aplikácie. Po prechode z Node.js na Go zostáva existujúca lokálna paleta zachovaná, ak používaš rovnakú doménu a prehliadač.

Technická dokumentácia: [Go embedded files](https://pkg.go.dev/embed), [OpenAI web search](https://developers.openai.com/api/docs/guides/tools-web-search), [obrazové vstupy](https://developers.openai.com/api/docs/guides/images-vision).
