# DashFetchr — Decizii deschise pentru kickoff

**Acesta este documentul de sign-off.** Inainte de a incepe development, toate deciziile de mai jos trebuie sa primeasca OK explicit de la Product Owner si Tech Lead.

Pentru fiecare decizie: optiuni, pros/cons, recomandare, decizia finala (de completat).

---

## D-01: Modelul principal pentru pilot

**Intrebare**: Lansam doar serviciul locker → casa (premium concierge) sau si home delivery direct (sameday→hub→home)?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Doar locker → casa** | Simplu, valoare clara, validare rapida, zero CAPEX | Pierdem segmentul "nu am locker, vreau direct acasa" |
| **B. Doar home delivery** (sameday→hub→home) | Adresam pain-ul "livrare ratata" mai general | Nevoie de hub-uri, complexitate operationala, CAPEX |
| **C. Ambele de la inceput** | Mai mult market | Spread thin, prea complex pentru MVP |

**Recomandare**: **A** pentru M1-M6, apoi adaugam **B** in M10-M12 ca upsell (folosind aceleasi rideri Bolt/Glovo si hub-uri partener Sameday).

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-02: Plata clientului final

**Intrebare**: Unde plateste clientul livrarea DashFetchr?

| Optiune | Pros | Cons |
|---|---|---|
| **A. In PWA DashFetchr** (Stripe + Netopia) | Brand DashFetchr, control complet, upsell posibil | UX cu un click in plus |
| **B. Inclus in pretul retailer la checkout** | Zero friction la livrare, retailerul colecteaza | Mai greu de negociat, retailerul devine gatekeeper |
| **C. Hybrid** | Flexibilitate | Complexitate dubla |

**Recomandare**: **A** pentru v1 (claritate, brand, learning). **C** in v2 daca apar retaileri mari care vor `B`.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-03: Acces la locker pentru pilot

**Intrebare**: Cum scoate riderul Bolt/Glovo coletul din easybox-ul Sameday?

| Optiune | Pros | Cons |
|---|---|---|
| **A. B2B Sameday partnership** (API access) | Sigur, scalabil, posibil moat | Necesita negociere prelungita, poate dura 3-6 luni |
| **B. PIN forwarding** (clientul ne da PIN-ul, il dam riderului) | Functioneaza azi, fara parteneriat | Posibil contrar T&C Sameday, chain of custody slabit, securitate |
| **C. Drop zone** (clientul scoate, lasa intr-o zona, riderul ridica) | Cel mai simplu | Strica propunerea de valoare (clientul tot coboara) |

**Recomandare**: lansam **B** in pilot (M1-M3) si negociem **A** in paralel din M0. Daca **A** intarzie peste M4, ramanem pe **B** si redam riscul legal cu consultant juridic.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ B → A &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-04: Primul last-mile carrier integrat

**Intrebare**: Pe care il integram primul: Bolt Food sau Glovo?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Bolt Food** | API mai stabil (raportat), penetrare buna Bucuresti, mai des dispus la parteneriate B2B | Mai putin acoperit in unele zone vs Glovo |
| **B. Glovo** | Acoperire mare, brand cunoscut "on-demand" | API mai inchis pentru aggregatori (istoric) |
| **C. Ambele in paralel** | Resilient | Dubla complexitate la pilot |

**Recomandare**: **A. Bolt Food**. **B. Glovo** ca al doilea in M5-M6.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-05: Geografie pilot

**Intrebare**: Unde lansam pilotul?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Doar Floreasca** | Focus maxim, marketing concentrat | Volum potential limitat |
| **B. Floreasca + Primaverii + Aviatorilor + Pipera** | Volum mai mare, demografice premium variate | Mai multe variabile operationale |
| **C. Sector 1 intreg** | Volum maxim | Diluare focus, mai greu de invata |

**Recomandare**: **A** in M1-M3 (60-80 easybox-uri tinta), extindere la **B** in M4. Sector 1 intreg in M6+.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-06: Limba interfata

**Intrebare**: PWA + admin dashboard in ce limbi?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Doar RO** | Simplu, focus | Pierdem expat segment (Floreasca are multi) |
| **B. RO + EN (toggle)** | Acopera expat segment | Cost localizare mai mare |
| **C. RO + EN auto-detect** | UX bun | Mai complex |

**Recomandare**: **B** de la inceput (i18n in foundation, doar text translations). EN-ul costa putin daca arhitectura e pregatita.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-07: Frontend stack

**Intrebare**: Next.js 14 (App Router) cu Tailwind + shadcn/ui?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Next.js 14 + Tailwind + shadcn/ui** (recomandat) | Standard, velocity, PWA support, ecosystem | — |
| **B. Vite + React + custom UI** | Mai lightweight | Mai mult cod custom |
| **C. SvelteKit** | Performant, modern | Echipa mai greu de gasit |

**Recomandare**: **A**.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-08: Backend language

**Intrebare**: Go pur sau Go + Node.js BFF?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Go pur** | Single language, simplitate operationala | Frontend dev trebuie sa atinga Go pentru BFF endpoints |
| **B. Go (core) + Node.js BFF** | Frontend devs fac BFF in TS, separare clara | 2 stacks de operat |
| **C. Node.js (TypeScript) tot** | Single stack, frontend devs scriu backend | Concurrency mai limitata, latenta mai mare la carrier API |

**Recomandare**: **A** pentru MVP. Adaugam un BFF Node.js doar daca echipa de frontend devine mare (M9+).

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-09: Strategie HR / echipa la kickoff

**Intrebare**: Cu cine pornim development-ul?

| Optiune | Pros | Cons | Cost lunar |
|---|---|---|---|
| **A. Minim viabil**: 1 backend senior + 1 frontend mid + 1 ops/founder | Burn redus | Bottleneck pe oameni | ~8-10k EUR |
| **B. Optim**: 1 backend senior + 1 backend mid + 1 frontend senior + 1 ops + 1 bizdev | Velocity, calitate | Burn mai mare | ~15-18k EUR |
| **C. Aggressive**: B + 1 designer + 1 QA | Polish maxim | Cash flow risk | ~20k EUR |

**Recomandare**: **A** pentru M1-M3 (pilot manual cu wizard of oz nu necesita echipa mare). **B** din M4 cand validare confirma.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-10: Cloud account + budget

**Intrebare**: Setup AWS si budget?

- AWS Organization: ☐ Da &nbsp;&nbsp; ☐ Nu (single account)
- Region: ☐ eu-central-1 (Frankfurt, recomandat) &nbsp;&nbsp; ☐ eu-west-1 (Irlanda) &nbsp;&nbsp; ☐ Alta
- Budget monthly limit cu alert: ____ EUR (recomandat: 200 EUR pentru M1-M3, 800 EUR pentru M4-M6)
- Cine e billing admin: ____________

**Decizie finala**: ____________

---

## D-11: Domeniu si branding

- Domeniu primar: ☐ dashfetchr.com &nbsp;&nbsp; ☐ dashfetchr.io &nbsp;&nbsp; ☐ Alt: ____________
- Logo & guidelines existente: ☐ Da &nbsp;&nbsp; ☐ Nu (necesita designer)
- WhatsApp Business display name: ____________
- SMS sender ID: ____________ (necesita aprobare la Twilio + carriere RO)

**Decizie finala**: ____________

---

## D-12: Identitate juridica si compliance

- Entitate juridica activa: ☐ SRL ____________ &nbsp;&nbsp; ☐ Trebuie creata
- Status juridic platforma: ☐ Intermediar tehnic (preferat) &nbsp;&nbsp; ☐ Transportator (mai costisitor)
- Consultant juridic: ____________
- DPO (Data Protection Officer): ____________
- Termeni & Conditii draft: ☐ Exista &nbsp;&nbsp; ☐ Trebuie redactati (cu juristul)

**Decizie finala**: ____________

---

## D-13: Parteneriate strategice — status

| Partener | Status conversation | Owner pe DashFetchr | Deadline conversie |
|---|---|---|---|
| Sameday | ☐ Nu inceput &nbsp;&nbsp; ☐ In curs &nbsp;&nbsp; ☐ LOI &nbsp;&nbsp; ☐ Contract | ____________ | ____________ |
| Bolt Food | ☐ Nu inceput &nbsp;&nbsp; ☐ In curs &nbsp;&nbsp; ☐ LOI &nbsp;&nbsp; ☐ Contract | ____________ | ____________ |
| Glovo | ☐ Nu inceput &nbsp;&nbsp; ☐ In curs &nbsp;&nbsp; ☐ LOI | ____________ | ____________ |
| Retailer pilot #1 | ☐ Nu inceput &nbsp;&nbsp; ☐ In curs &nbsp;&nbsp; ☐ LOI | ____________ | ____________ |
| Retailer pilot #2 | ☐ Nu inceput &nbsp;&nbsp; ☐ In curs &nbsp;&nbsp; ☐ LOI | ____________ | ____________ |

**Recomandare**: **minim 1 LOI Sameday + 1 LOI Bolt + 1 LOI retailer** inainte de a scrie cod productie.

---

## D-14: Validare pre-development

**Intrebare**: Cat de mult validam inainte de cod?

| Optiune | Pros | Cons |
|---|---|---|
| **A. Sondaj + concierge manual** (200 raspunsuri + 20 livrari manuale) | Validare reala a cererii si UX | 2-3 saptamani extra |
| **B. Doar interviuri (10 persoane) si pornim development** | Velocity | Risc sa construim ce nu se vinde |
| **C. Wizard of oz live cu cod minim si traffic real** | Validare in conditii reale | Mai mult risk operational |

**Recomandare**: **A**, fie chiar in paralel cu D-13 (parteneriate). Validarea iti da ammunition pentru a negocia.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## D-15: Bugetul de timp pentru kickoff M1

**Intrebare**: Cand vrem prima livrare reala in prod?

| Optiune | Implicatii |
|---|---|
| **A. 6 saptamani de la sign-off** | Foarte rapid, MVP minimal, multe shortcut-uri |
| **B. 10-12 saptamani** | Realistic pentru MVP cu calitate buna |
| **C. 16 saptamani** | Confortabil, polish bun |

**Recomandare**: **B**. Suficient cat sa nu fie precipitat, dar fara sa pierdem momentumul.

**Decizie finala**: ☐ A &nbsp;&nbsp; ☐ B &nbsp;&nbsp; ☐ C &nbsp;&nbsp; ☐ Alta: ____________

---

## Sign-off

Dupa ce toate deciziile de mai sus au raspuns:

| Rol | Nume | Data | Semnatura |
|---|---|---|---|
| Product Owner | _______________ | __________ | __________ |
| Tech Lead | _______________ | __________ | __________ |
| Founder / CEO | _______________ | __________ | __________ |

**De la momentul tuturor sign-off-urilor, lucrul tehnic pe MVP poate incepe oficial.**

---

## Anexa: Decizii "lock & load" (luate de tech)

Aceste decizii sunt luate de tech lead ca defaults rezonabile si nu necesita aprobare formala, dar pot fi contestate:

- Backend: Go 1.22 cu chi + sqlc + pgx
- DB: PostgreSQL 16, single primary + read replica din M6
- Storage: AWS S3 cu Object Lock (compliance mode, 7 ani retention pentru poze POD)
- Queue: AWS SQS (Kafka in v2 daca volumul cere)
- Region: AWS eu-central-1 (Frankfurt)
- Logging: structured slog (stdlib)
- Tracing: OpenTelemetry
- Frontend: Next.js 14 + Tailwind + shadcn/ui
- Plata: Stripe (international cards) + Netopia (local cards RO)
- SMS/WhatsApp: Twilio (SMS) + 360dialog sau Twilio (WhatsApp Business)
- Maps: Mapbox

Daca PO sau Tech Lead vor sa overrideze vreo decizie de mai sus, mentioneaza explicit in sectiunea ta de comentarii la review.

---

## Comentarii reviewer

(Lasati comentarii aici, ne intalnim sa le discutam)

**PO**:
> 

**Tech Lead**:
> 

**Founder**:
> 
