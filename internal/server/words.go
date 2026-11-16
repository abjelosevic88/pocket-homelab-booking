package server

import (
	"fmt"
	"time"
	"unicode"
)

// siteWords are the words the public pages and the guests' emails say
// themselves, per language (page titles and questions are the owner's own).
var siteWords = map[string]map[string]string{
	"en": {
		"lang": "English", "pick_time": "Pick a time", "pick_day": "Pick a day", "no_times": "No free times in this month.",
		"no_times_day": "No free times on this day.", "next": "Later", "prev": "Earlier", "times_in": "Times are in %s.", "change_tz": "Show in another time zone",
		"show": "Show", "min": "%d min", "back": "Back", "details": "Your details", "name": "Your name", "email": "Email", "phone": "Phone",
		"where_q": "Where should we meet?", "book": "Book", "request": "Ask for this time", "required": "Required", "optional": "optional", "choose": "Choose…",
		"yes": "Yes", "no": "No", "paused_title": "Not taking bookings right now", "paused": "Please come back a little later.",
		"notfound": "There is no booking page at this address.", "preview": "Preview — only you see this. Open the page to take bookings.",
		"preview_sent": "This was a preview, so nothing was booked.", "taken": "Someone has just booked that time. Please pick another one.",
		"e_required": "Please fill this in.", "e_email": "This does not look like an email address.", "e_phone": "This does not look like a phone number.",
		"e_number": "Please write a number.", "e_url": "Please write a web address.", "e_date": "Please pick a day.", "e_choice": "Please pick one of the choices.",
		"e_long": "This is too long.", "fix": "Please look at the fields marked below.", "wait": "Many bookings came from here just now. Please try again in a few minutes.",
		"stamp": "The page was open for a long time, so please send it once more.", "made": "Made with Pocket Booking",
		"booked_title": "You're booked", "booked": "A confirmation is on its way to %s.", "booked_noemail": "Keep this page: it is your booking.",
		"pending_title": "Request sent", "pending": "%s will confirm it soon. You'll get an email.", "cancelled_title": "This booking is cancelled",
		"declined_title": "This time is not possible", "over_title": "This booking is over", "moved_title": "Your booking has a new time",
		"when": "When", "where": "Where", "with": "With", "length": "How long", "price": "Price", "pay": "How to pay", "add_cal": "Add to my calendar",
		"move": "Pick another time", "cancel": "Cancel the booking", "cancel_why": "Anything to say? (optional)", "cancel_yes": "Yes, cancel it",
		"keep": "Keep it", "moving": "Pick the new time", "video": "Video call", "join": "Join the call", "phone_out": "%s will call you",
		"phone_in": "Call %s", "places_left": "%d places left", "how_long": "How long?", "for_you": "This link is for %s.",
		"invite_used": "This personal link has already been used.", "invite_gone": "This personal link has run out. Please ask for a new one.",
		"pages_of": "Book a time with %s", "no_pages": "There is nothing to book here right now.", "book_this": "Pick a time",
		"too_late": "This booking has already started, so it cannot be changed here.", "cancel_done": "Your booking is cancelled.",
		"by_owner": "%s cancelled this booking.", "reason": "Message", "book_again": "Book another time", "today": "Today", "tomorrow": "Tomorrow",
		"m_booked": "Booked: %s", "m_booked_body": "Your booking is confirmed. See you then!", "m_asked": "Request received: %s",
		"m_asked_body": "Thank you! %s will confirm your booking soon; you'll get another email.", "m_confirmed": "Confirmed: %s",
		"m_confirmed_body": "Good news: your booking is confirmed.", "m_declined": "Not possible: %s",
		"m_declined_body": "Sorry, %s cannot take this booking. You are welcome to pick another time.", "m_cancelled": "Cancelled: %s",
		"m_cancelled_body": "This booking is cancelled.", "m_moved": "New time: %s", "m_moved_body": "Your booking has a new time.",
		"m_reminder": "Reminder: %s", "m_reminder_body": "A short reminder of your booking.", "m_manage": "To move or cancel it, open your booking:",
		"m_open": "Open your booking", "m_message": "A message from %s:",
	},
	"bs": {
		"lang": "Bosanski", "pick_time": "Odaberite termin", "pick_day": "Odaberite dan", "no_times": "Ovaj mjesec nema slobodnih termina.",
		"no_times_day": "Ovaj dan nema slobodnih termina.", "next": "Kasnije", "prev": "Ranije", "times_in": "Vremena su prikazana za %s.", "change_tz": "Prikaži u drugoj vremenskoj zoni",
		"show": "Prikaži", "min": "%d min", "back": "Nazad", "details": "Vaši podaci", "name": "Ime i prezime", "email": "E-mail", "phone": "Telefon",
		"where_q": "Gdje da se nađemo?", "book": "Rezerviši", "request": "Zatraži ovaj termin", "required": "Obavezno", "optional": "nije obavezno", "choose": "Odaberite…",
		"yes": "Da", "no": "Ne", "paused_title": "Trenutno nema rezervacija", "paused": "Molimo navratite malo kasnije.",
		"notfound": "Na ovoj adresi nema stranice za rezervacije.", "preview": "Pregled — vidite ga samo vi. Otvorite stranicu da bi primala rezervacije.",
		"preview_sent": "Ovo je bio pregled, ništa nije rezervisano.", "taken": "Neko je upravo rezervisao taj termin. Molimo odaberite drugi.",
		"e_required": "Molimo popunite ovo polje.", "e_email": "Ovo ne izgleda kao e-mail adresa.", "e_phone": "Ovo ne izgleda kao broj telefona.",
		"e_number": "Molimo upišite broj.", "e_url": "Molimo upišite web adresu.", "e_date": "Molimo odaberite dan.", "e_choice": "Molimo odaberite jedan od ponuđenih odgovora.",
		"e_long": "Ovo je predugo.", "fix": "Molimo pogledajte označena polja.", "wait": "Odavde je upravo stiglo mnogo rezervacija. Pokušajte ponovo za nekoliko minuta.",
		"stamp": "Stranica je dugo bila otvorena, molimo pošaljite još jednom.", "made": "Napravljeno u Pocket Booking",
		"booked_title": "Rezervisano je", "booked": "Potvrda stiže na %s.", "booked_noemail": "Sačuvajte ovu stranicu: to je vaša rezervacija.",
		"pending_title": "Zahtjev je poslan", "pending": "%s će ga uskoro potvrditi. Dobit ćete e-mail.", "cancelled_title": "Ova rezervacija je otkazana",
		"declined_title": "Ovaj termin nije moguć", "over_title": "Ova rezervacija je prošla", "moved_title": "Vaša rezervacija ima novi termin",
		"when": "Kada", "where": "Gdje", "with": "S kim", "length": "Trajanje", "price": "Cijena", "pay": "Kako platiti", "add_cal": "Dodaj u moj kalendar",
		"move": "Odaberi drugi termin", "cancel": "Otkaži rezervaciju", "cancel_why": "Želite li nešto poručiti? (nije obavezno)", "cancel_yes": "Da, otkaži",
		"keep": "Zadrži", "moving": "Odaberite novi termin", "video": "Video poziv", "join": "Uđi u poziv", "phone_out": "%s će vas nazvati",
		"phone_in": "Nazovite %s", "places_left": "Slobodnih mjesta: %d", "how_long": "Koliko dugo?", "for_you": "Ovaj link je za: %s.",
		"invite_used": "Ovaj lični link je već iskorišten.", "invite_gone": "Ovaj lični link je istekao. Molimo zatražite novi.",
		"pages_of": "Rezervišite termin kod: %s", "no_pages": "Trenutno ovdje nema ničega za rezervisati.", "book_this": "Odaberite termin",
		"too_late": "Ova rezervacija je već počela, pa se ovdje ne može mijenjati.", "cancel_done": "Vaša rezervacija je otkazana.",
		"by_owner": "%s je otkazao/la ovu rezervaciju.", "reason": "Poruka", "book_again": "Rezerviši drugi termin", "today": "Danas", "tomorrow": "Sutra",
		"m_booked": "Rezervisano: %s", "m_booked_body": "Vaša rezervacija je potvrđena. Vidimo se!", "m_asked": "Zahtjev primljen: %s",
		"m_asked_body": "Hvala! %s će uskoro potvrditi vašu rezervaciju; dobit ćete još jedan e-mail.", "m_confirmed": "Potvrđeno: %s",
		"m_confirmed_body": "Dobre vijesti: vaša rezervacija je potvrđena.", "m_declined": "Nije moguće: %s",
		"m_declined_body": "Nažalost, %s ne može prihvatiti ovu rezervaciju. Slobodno odaberite drugi termin.", "m_cancelled": "Otkazano: %s",
		"m_cancelled_body": "Ova rezervacija je otkazana.", "m_moved": "Novi termin: %s", "m_moved_body": "Vaša rezervacija ima novi termin.",
		"m_reminder": "Podsjetnik: %s", "m_reminder_body": "Kratki podsjetnik na vašu rezervaciju.", "m_manage": "Da je pomjerite ili otkažete, otvorite svoju rezervaciju:",
		"m_open": "Otvori rezervaciju", "m_message": "Poruka od: %s",
	},
	"hr": {
		"lang": "Hrvatski", "pick_time": "Odaberite termin", "pick_day": "Odaberite dan", "no_times": "Ovaj mjesec nema slobodnih termina.",
		"no_times_day": "Ovaj dan nema slobodnih termina.", "next": "Kasnije", "prev": "Ranije", "times_in": "Vremena su prikazana za %s.", "change_tz": "Prikaži u drugoj vremenskoj zoni",
		"show": "Prikaži", "min": "%d min", "back": "Natrag", "details": "Vaši podaci", "name": "Ime i prezime", "email": "E-pošta", "phone": "Telefon",
		"where_q": "Gdje da se nađemo?", "book": "Rezerviraj", "request": "Zatraži ovaj termin", "required": "Obavezno", "optional": "nije obavezno", "choose": "Odaberite…",
		"yes": "Da", "no": "Ne", "paused_title": "Trenutačno nema rezervacija", "paused": "Molimo navratite malo kasnije.",
		"notfound": "Na ovoj adresi nema stranice za rezervacije.", "preview": "Pregled — vidite ga samo vi. Otvorite stranicu da bi primala rezervacije.",
		"preview_sent": "Ovo je bio pregled, ništa nije rezervirano.", "taken": "Netko je upravo rezervirao taj termin. Molimo odaberite drugi.",
		"e_required": "Molimo ispunite ovo polje.", "e_email": "Ovo ne izgleda kao adresa e-pošte.", "e_phone": "Ovo ne izgleda kao broj telefona.",
		"e_number": "Molimo upišite broj.", "e_url": "Molimo upišite web-adresu.", "e_date": "Molimo odaberite dan.", "e_choice": "Molimo odaberite jedan od ponuđenih odgovora.",
		"e_long": "Ovo je predugo.", "fix": "Molimo pogledajte označena polja.", "wait": "Odavde je upravo stiglo mnogo rezervacija. Pokušajte ponovno za nekoliko minuta.",
		"stamp": "Stranica je dugo bila otvorena, molimo pošaljite još jednom.", "made": "Izrađeno u Pocket Booking",
		"booked_title": "Rezervirano je", "booked": "Potvrda stiže na %s.", "booked_noemail": "Spremite ovu stranicu: to je vaša rezervacija.",
		"pending_title": "Zahtjev je poslan", "pending": "%s će ga uskoro potvrditi. Dobit ćete e-poruku.", "cancelled_title": "Ova je rezervacija otkazana",
		"declined_title": "Ovaj termin nije moguć", "over_title": "Ova je rezervacija prošla", "moved_title": "Vaša rezervacija ima novi termin",
		"when": "Kada", "where": "Gdje", "with": "S kim", "length": "Trajanje", "price": "Cijena", "pay": "Kako platiti", "add_cal": "Dodaj u moj kalendar",
		"move": "Odaberi drugi termin", "cancel": "Otkaži rezervaciju", "cancel_why": "Želite li nešto poručiti? (nije obavezno)", "cancel_yes": "Da, otkaži",
		"keep": "Zadrži", "moving": "Odaberite novi termin", "video": "Videopoziv", "join": "Uđi u poziv", "phone_out": "%s će vas nazvati",
		"phone_in": "Nazovite %s", "places_left": "Slobodnih mjesta: %d", "how_long": "Koliko dugo?", "for_you": "Ova je poveznica za: %s.",
		"invite_used": "Ova je osobna poveznica već iskorištena.", "invite_gone": "Ova je osobna poveznica istekla. Molimo zatražite novu.",
		"pages_of": "Rezervirajte termin kod: %s", "no_pages": "Trenutačno ovdje nema ničega za rezervirati.", "book_this": "Odaberite termin",
		"too_late": "Ova je rezervacija već počela pa se ovdje ne može mijenjati.", "cancel_done": "Vaša je rezervacija otkazana.",
		"by_owner": "%s je otkazao/la ovu rezervaciju.", "reason": "Poruka", "book_again": "Rezerviraj drugi termin", "today": "Danas", "tomorrow": "Sutra",
		"m_booked": "Rezervirano: %s", "m_booked_body": "Vaša je rezervacija potvrđena. Vidimo se!", "m_asked": "Zahtjev primljen: %s",
		"m_asked_body": "Hvala! %s će uskoro potvrditi vašu rezervaciju; dobit ćete još jednu e-poruku.", "m_confirmed": "Potvrđeno: %s",
		"m_confirmed_body": "Dobre vijesti: vaša je rezervacija potvrđena.", "m_declined": "Nije moguće: %s",
		"m_declined_body": "Nažalost, %s ne može prihvatiti ovu rezervaciju. Slobodno odaberite drugi termin.", "m_cancelled": "Otkazano: %s",
		"m_cancelled_body": "Ova je rezervacija otkazana.", "m_moved": "Novi termin: %s", "m_moved_body": "Vaša rezervacija ima novi termin.",
		"m_reminder": "Podsjetnik: %s", "m_reminder_body": "Kratki podsjetnik na vašu rezervaciju.", "m_manage": "Da je pomaknete ili otkažete, otvorite svoju rezervaciju:",
		"m_open": "Otvori rezervaciju", "m_message": "Poruka od: %s",
	},
	"sr": {
		"lang": "Srpski", "pick_time": "Izaberite termin", "pick_day": "Izaberite dan", "no_times": "Ovaj mesec nema slobodnih termina.",
		"no_times_day": "Ovaj dan nema slobodnih termina.", "next": "Kasnije", "prev": "Ranije", "times_in": "Vremena su prikazana za %s.", "change_tz": "Prikaži u drugoj vremenskoj zoni",
		"show": "Prikaži", "min": "%d min", "back": "Nazad", "details": "Vaši podaci", "name": "Ime i prezime", "email": "Imejl", "phone": "Telefon",
		"where_q": "Gde da se nađemo?", "book": "Rezerviši", "request": "Zatraži ovaj termin", "required": "Obavezno", "optional": "nije obavezno", "choose": "Izaberite…",
		"yes": "Da", "no": "Ne", "paused_title": "Trenutno nema rezervacija", "paused": "Molimo navratite malo kasnije.",
		"notfound": "Na ovoj adresi nema stranice za rezervacije.", "preview": "Pregled — vidite ga samo vi. Otvorite stranicu da bi primala rezervacije.",
		"preview_sent": "Ovo je bio pregled, ništa nije rezervisano.", "taken": "Neko je upravo rezervisao taj termin. Molimo izaberite drugi.",
		"e_required": "Molimo popunite ovo polje.", "e_email": "Ovo ne izgleda kao imejl adresa.", "e_phone": "Ovo ne izgleda kao broj telefona.",
		"e_number": "Molimo upišite broj.", "e_url": "Molimo upišite veb adresu.", "e_date": "Molimo izaberite dan.", "e_choice": "Molimo izaberite jedan od ponuđenih odgovora.",
		"e_long": "Ovo je predugačko.", "fix": "Molimo pogledajte označena polja.", "wait": "Odavde je upravo stiglo mnogo rezervacija. Pokušajte ponovo za nekoliko minuta.",
		"stamp": "Stranica je dugo bila otvorena, molimo pošaljite još jednom.", "made": "Napravljeno u Pocket Booking",
		"booked_title": "Rezervisano je", "booked": "Potvrda stiže na %s.", "booked_noemail": "Sačuvajte ovu stranicu: to je vaša rezervacija.",
		"pending_title": "Zahtev je poslat", "pending": "%s će ga uskoro potvrditi. Dobićete imejl.", "cancelled_title": "Ova rezervacija je otkazana",
		"declined_title": "Ovaj termin nije moguć", "over_title": "Ova rezervacija je prošla", "moved_title": "Vaša rezervacija ima novi termin",
		"when": "Kada", "where": "Gde", "with": "Sa kim", "length": "Trajanje", "price": "Cena", "pay": "Kako platiti", "add_cal": "Dodaj u moj kalendar",
		"move": "Izaberi drugi termin", "cancel": "Otkaži rezervaciju", "cancel_why": "Želite li nešto da poručite? (nije obavezno)", "cancel_yes": "Da, otkaži",
		"keep": "Zadrži", "moving": "Izaberite novi termin", "video": "Video poziv", "join": "Uđi u poziv", "phone_out": "%s će vas pozvati",
		"phone_in": "Pozovite %s", "places_left": "Slobodnih mesta: %d", "how_long": "Koliko dugo?", "for_you": "Ovaj link je za: %s.",
		"invite_used": "Ovaj lični link je već iskorišćen.", "invite_gone": "Ovaj lični link je istekao. Molimo zatražite novi.",
		"pages_of": "Rezervišite termin kod: %s", "no_pages": "Trenutno ovde nema ničega za rezervisanje.", "book_this": "Izaberite termin",
		"too_late": "Ova rezervacija je već počela, pa se ovde ne može menjati.", "cancel_done": "Vaša rezervacija je otkazana.",
		"by_owner": "%s je otkazao/la ovu rezervaciju.", "reason": "Poruka", "book_again": "Rezerviši drugi termin", "today": "Danas", "tomorrow": "Sutra",
		"m_booked": "Rezervisano: %s", "m_booked_body": "Vaša rezervacija je potvrđena. Vidimo se!", "m_asked": "Zahtev primljen: %s",
		"m_asked_body": "Hvala! %s će uskoro potvrditi vašu rezervaciju; dobićete još jedan imejl.", "m_confirmed": "Potvrđeno: %s",
		"m_confirmed_body": "Dobre vesti: vaša rezervacija je potvrđena.", "m_declined": "Nije moguće: %s",
		"m_declined_body": "Nažalost, %s ne može da prihvati ovu rezervaciju. Slobodno izaberite drugi termin.", "m_cancelled": "Otkazano: %s",
		"m_cancelled_body": "Ova rezervacija je otkazana.", "m_moved": "Novi termin: %s", "m_moved_body": "Vaša rezervacija ima novi termin.",
		"m_reminder": "Podsetnik: %s", "m_reminder_body": "Kratak podsetnik na vašu rezervaciju.", "m_manage": "Da je pomerite ili otkažete, otvorite svoju rezervaciju:",
		"m_open": "Otvori rezervaciju", "m_message": "Poruka od: %s",
	},
	"de": {
		"lang": "Deutsch", "pick_time": "Termin wählen", "pick_day": "Tag wählen", "no_times": "In diesem Monat sind keine Termine frei.",
		"no_times_day": "An diesem Tag sind keine Termine frei.", "next": "Später", "prev": "Früher", "times_in": "Zeiten in %s.", "change_tz": "In einer anderen Zeitzone zeigen",
		"show": "Zeigen", "min": "%d Min.", "back": "Zurück", "details": "Ihre Angaben", "name": "Ihr Name", "email": "E-Mail", "phone": "Telefon",
		"where_q": "Wo sollen wir uns treffen?", "book": "Buchen", "request": "Diesen Termin anfragen", "required": "Pflichtfeld", "optional": "freiwillig", "choose": "Bitte wählen…",
		"yes": "Ja", "no": "Nein", "paused_title": "Gerade keine Buchungen", "paused": "Bitte schauen Sie etwas später wieder vorbei.",
		"notfound": "Unter dieser Adresse gibt es keine Buchungsseite.", "preview": "Vorschau — nur Sie sehen das. Öffnen Sie die Seite, um Buchungen anzunehmen.",
		"preview_sent": "Das war eine Vorschau, es wurde nichts gebucht.", "taken": "Dieser Termin wurde gerade gebucht. Bitte wählen Sie einen anderen.",
		"e_required": "Bitte füllen Sie das aus.", "e_email": "Das sieht nicht wie eine E-Mail-Adresse aus.", "e_phone": "Das sieht nicht wie eine Telefonnummer aus.",
		"e_number": "Bitte geben Sie eine Zahl ein.", "e_url": "Bitte geben Sie eine Webadresse ein.", "e_date": "Bitte wählen Sie einen Tag.", "e_choice": "Bitte wählen Sie eine der Antworten.",
		"e_long": "Das ist zu lang.", "fix": "Bitte sehen Sie sich die markierten Felder an.", "wait": "Von hier kamen gerade viele Buchungen. Bitte versuchen Sie es in ein paar Minuten noch einmal.",
		"stamp": "Die Seite war lange offen, bitte senden Sie sie noch einmal.", "made": "Erstellt mit Pocket Booking",
		"booked_title": "Sie sind gebucht", "booked": "Eine Bestätigung geht an %s.", "booked_noemail": "Bewahren Sie diese Seite auf: Das ist Ihre Buchung.",
		"pending_title": "Anfrage gesendet", "pending": "%s bestätigt sie bald. Sie bekommen eine E-Mail.", "cancelled_title": "Diese Buchung ist abgesagt",
		"declined_title": "Dieser Termin ist nicht möglich", "over_title": "Diese Buchung ist vorbei", "moved_title": "Ihre Buchung hat einen neuen Termin",
		"when": "Wann", "where": "Wo", "with": "Mit", "length": "Dauer", "price": "Preis", "pay": "Bezahlung", "add_cal": "In meinen Kalender",
		"move": "Anderen Termin wählen", "cancel": "Buchung absagen", "cancel_why": "Möchten Sie etwas sagen? (freiwillig)", "cancel_yes": "Ja, absagen",
		"keep": "Behalten", "moving": "Neuen Termin wählen", "video": "Videoanruf", "join": "Am Anruf teilnehmen", "phone_out": "%s ruft Sie an",
		"phone_in": "Rufen Sie %s an", "places_left": "Noch %d Plätze", "how_long": "Wie lange?", "for_you": "Dieser Link ist für %s.",
		"invite_used": "Dieser persönliche Link wurde schon benutzt.", "invite_gone": "Dieser persönliche Link ist abgelaufen. Bitte fragen Sie nach einem neuen.",
		"pages_of": "Einen Termin mit %s buchen", "no_pages": "Hier gibt es gerade nichts zu buchen.", "book_this": "Termin wählen",
		"too_late": "Diese Buchung hat schon begonnen und kann hier nicht mehr geändert werden.", "cancel_done": "Ihre Buchung ist abgesagt.",
		"by_owner": "%s hat diese Buchung abgesagt.", "reason": "Nachricht", "book_again": "Einen anderen Termin buchen", "today": "Heute", "tomorrow": "Morgen",
		"m_booked": "Gebucht: %s", "m_booked_body": "Ihre Buchung ist bestätigt. Bis dann!", "m_asked": "Anfrage erhalten: %s",
		"m_asked_body": "Danke! %s bestätigt Ihre Buchung bald; Sie bekommen noch eine E-Mail.", "m_confirmed": "Bestätigt: %s",
		"m_confirmed_body": "Gute Nachricht: Ihre Buchung ist bestätigt.", "m_declined": "Nicht möglich: %s",
		"m_declined_body": "Leider kann %s diese Buchung nicht annehmen. Wählen Sie gern einen anderen Termin.", "m_cancelled": "Abgesagt: %s",
		"m_cancelled_body": "Diese Buchung ist abgesagt.", "m_moved": "Neuer Termin: %s", "m_moved_body": "Ihre Buchung hat einen neuen Termin.",
		"m_reminder": "Erinnerung: %s", "m_reminder_body": "Eine kurze Erinnerung an Ihre Buchung.", "m_manage": "Zum Verschieben oder Absagen öffnen Sie Ihre Buchung:",
		"m_open": "Buchung öffnen", "m_message": "Eine Nachricht von %s:",
	},
}

var weekdays = map[string][7]string{
	"en": {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	"bs": {"nedjelja", "ponedjeljak", "utorak", "srijeda", "četvrtak", "petak", "subota"},
	"hr": {"nedjelja", "ponedjeljak", "utorak", "srijeda", "četvrtak", "petak", "subota"},
	"sr": {"nedelja", "ponedeljak", "utorak", "sreda", "četvrtak", "petak", "subota"},
	"de": {"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"},
}

var shortDays = map[string][7]string{
	"en": {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	"bs": {"ned", "pon", "uto", "sri", "čet", "pet", "sub"},
	"hr": {"ned", "pon", "uto", "sri", "čet", "pet", "sub"},
	"sr": {"ned", "pon", "uto", "sre", "čet", "pet", "sub"},
	"de": {"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"},
}

var months = map[string][12]string{
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	"bs": {"januar", "februar", "mart", "april", "maj", "juni", "juli", "august", "septembar", "oktobar", "novembar", "decembar"},
	"hr": {"siječanj", "veljača", "ožujak", "travanj", "svibanj", "lipanj", "srpanj", "kolovoz", "rujan", "listopad", "studeni", "prosinac"},
	"sr": {"januar", "februar", "mart", "april", "maj", "jun", "jul", "avgust", "septembar", "oktobar", "novembar", "decembar"},
	"de": {"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
}

// Croatian months inside a date ("18. studenoga") take another ending.
var monthsOf = [12]string{"siječnja", "veljače", "ožujka", "travnja", "svibnja", "lipnja", "srpnja", "kolovoza", "rujna", "listopada", "studenoga", "prosinca"}

// word is one word in a language (English when the language lacks it).
func word(lang, key string) string {
	if w, ok := siteWords[lang][key]; ok {
		return w
	}
	return siteWords["en"][key]
}

func langOK(lang string) string {
	if _, ok := siteWords[lang]; ok {
		return lang
	}
	return "en"
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// dayName is a day in words: "Wednesday 18 November 2026", "srijeda, 18. novembar 2026.".
func dayName(t time.Time, lang string) string {
	lang = langOK(lang)
	wd, mo := weekdays[lang][t.Weekday()], months[lang][t.Month()-1]
	switch lang {
	case "en":
		return fmt.Sprintf("%s %d %s %d", wd, t.Day(), mo, t.Year())
	case "de":
		return fmt.Sprintf("%s, %d. %s %d", wd, t.Day(), mo, t.Year())
	case "hr":
		return upperFirst(fmt.Sprintf("%s, %d. %s %d.", wd, t.Day(), monthsOf[t.Month()-1], t.Year()))
	}
	return upperFirst(fmt.Sprintf("%s, %d. %s %d.", wd, t.Day(), mo, t.Year()))
}

// shortDay is "Wed 18" in a language.
func shortDay(t time.Time, lang string) string {
	lang = langOK(lang)
	if lang == "en" {
		return fmt.Sprintf("%s %d", shortDays[lang][t.Weekday()], t.Day())
	}
	return fmt.Sprintf("%s %d.", upperFirst(shortDays[lang][t.Weekday()]), t.Day())
}

// monthName is "November 2026" in a language.
func monthName(t time.Time, lang string) string {
	lang = langOK(lang)
	return fmt.Sprintf("%s %d", upperFirst(months[lang][t.Month()-1]), t.Year())
}
