# IIM Ahmedabad Garba passes

_Started 2026-09-23 18:58 UTC_

---

## User

Make UI mockups for garba passes website at IIM Ahmedabad - organised by cultcomm. Make it indian gujarati atesthetic and pretty and very aesthetic - the website will be used for passes distribution and management. You can get the logos from the internet if you require any.

The website will be mobile first website design and will be used by all users for passes manahement. make the design aesthetic and beautiful and indian and navratri and garba themed

## Assistant



## Assistant

_[tool: ask_user]_

_[tool: dc_write]_

_[tool: ready_for_verification]_

I've made 8 mobile screens in **Garba Passes.dc.html**: landing, sign-in, choose pass, checkout, my pass (entry QR), invite guest, volunteer gate scanner and Cultcom admin.

- **Look:** a deep indigo night sky with kumkum red and marigold. The patterns are bandhani dots, toran borders and mirror-work details. The headings use a Gujarati typeface (ગરબા, કેમ છો!).
- **What you can click:** on the choose-pass screen, tapping a pass card selects it and updates the total. The scanner has a switch in Tweaks between "Entry allowed" and "Already used". Everything else is a static mockup.

You didn't answer the opening questions, so I made up the details. Please correct these:
- **Dates and venue:** 11–19 Oct 2026 at Louis Kahn Plaza. Please check the dates against the real Navratri calendar.
- **Prices:** ₹249 for one night, ₹1,499 for the season pass, ₹400 for a guest pass.
- **The colour for each night** is a common tradition, not official for 2026.
- **The numbers on the admin screen** are placeholders.

I didn't fetch the IIMA or Cultcom logos, so there are clearly marked logo spaces instead. The photo, UPI app logos and QR code are placeholders too. Upload the real logos and a photo and I'll add them.

## User

Found issues — fixing…

## Assistant

_[tool: dc_html_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: ready_for_verification]_

On the landing screen, the row of 9 nights now fits the width, so the last night (19 Mon) is no longer cut off. It opens with no errors, but I haven't taken a new look at it.

## User

Answered — screens: Landing / event info, Login (IIMA email / OTP), Pass types & booking, My passes (QR ticket), Entry scanner (volunteer); users: PGP students, Faculty & staff, External guests, Cultcom volunteers, Cultcom admins; passes: No prices - all iima students and staff get N number of passes (depends on if they are 1st year, 2nd year or staff etc) and they can send passes to their friends whoever they want. For external guest…; details: IIMA Garba Night - 1 night only; variations: 2 directions; motifs: Bandhani dots, Kutch embroidery, Dandiya sticks, Chaniya choli colours, Rangoli; p…

## Assistant

_[tool: view_image]_

_[tool: image_metadata]_

_[tool: copy_files]_

_[tool: dc_write]_

_[tool: ready_for_verification]_

I've rebuilt the site as **Garba Passes v2.dc.html** for your one-night event with pass quotas and no payments. There are two clickable directions, both using your Cultcomm logo. It's out for a check now.

- **2a Bandhani:** kumkum red with tie-dye dots, a rangoli on the landing page and crossed dandiya sticks.
- **2b Kutch:** ivory and black with Kutch embroidery borders, mirror work and chaniya-style stripes in your logo's colours.

A switch above each phone changes the user type:
- **IIMA student:** landing page, then sign-in, then your own pass and how many you can still share. You can type a friend's name and send them a pass; it appears in your list and the count goes down. Your pass opens to its entry QR.
- **Exchange guest:** a pass already in their name, marked as coming through the pass exchange.
- **Volunteer:** the gate scanner. Tap "Scan next" to go through the results: allowed, allowed exchange guest, and already used.

These details are placeholders for you to confirm:
- **Event:** Sat 17 Oct at Louis Kahn Plaza from 8 PM.
- **Quota:** 4 passes for a PGP1 student. Tell me the real numbers for PGP1, PGP2 and staff.
- **Names:** Aarav (student), Ishaan from SPJIMR (exchange guest), and the friends in the sent list.

Two things you listed aren't built yet: separate quotas per group, and a Cultcomm admin screen for uploading the exchange college lists. Tell me if you want them. The first version, with prices and 9 nights, is still in **Garba Passes.dc.html**.

## User

Found issues — fixing…

## Assistant

_[tool: dc_html_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: dc_html_str_replace]_

_[tool: ready_for_verification]_

I fixed both problems the check found. The file opens with no errors; I skipped a second full check since these were small edits.

- **2a readability:** small text on the red dotted background was hard to read. The date and venue line, the pass-exchange note on the landing page and the two notes on the guest screen now sit on a solid dark-red backing, at full brightness.
- **2b "Send a pass" count:** the box showed two different numbers. It now reads as one statement: how many passes are left after this one goes out.

## User

I like 2b - delete 2a

## Assistant

_[tool: run_script]_

_[tool: grep]_

Only colour hex matches remain.

_[tool: ready_for_verification]_

I deleted 2a, so the page now shows only the Kutch design, 2b. It opens with no errors and all the clickable flows are unchanged.

