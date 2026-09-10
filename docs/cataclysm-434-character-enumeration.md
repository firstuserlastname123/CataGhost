# Cataclysm 15595 character enumeration

Authority: local TCPP aa47817dfb, CharacterPackets.cpp:148-239 and
CharacterPackets.h:32-101; CharacterHandler.cpp:244-310; WorldPacket.cpp.
Secondary: Clientless 85422ae CharacterList.cpp and CharacterHandler.cpp.
WowPacketParser V4_3_4_15595 fetch was unavailable (cache miss).

Request CMSG_ENUM_CHARACTERS = 0x0502; empty body. After unqueued AUTH_OK,
write a six-byte header (BE16 size=4, LE32 opcode) encrypted with the client
RC4 stream (HMAC-SHA1 seed c2b3723cc6aed9b5343c53ee2f4367ce, drop1024).
Response SMSG_ENUM_CHARACTERS_RESULT = 0x10B0. Reuse the live receive cipher.
Do not send realm split, account-data requests, character login or gameplay.

Response bits, MSB first: restriction count(23), Success(1), character count(17).
For EACH character, without alignment between characters:
G3,H1,H7,H2,name byte length(7),G4,G7,H3,G5,H6,G1,H5,H4,FirstLogin,
G0,G2,G6,H0. G=character GUID; H=guild GUID. These GUID bits mark nonzero
bytes. Flush once after all character metadata. GUID bytes in the body are
present only when their mask is set and contain the actual byte XOR 1.

Then EACH character body, all numbers little-endian:
Class(u8); 23*(InvType(u8),DisplayID(u32),DisplayEnchantID(u32));
PetFamily(u32); H2; ListPosition(u8); HairStyle(u8); H3; PetDisplay(u32);
Flags(u32); HairColor(u8); G4; Map(i32); H5; Z(f32); H6; PetLevel(u32);
G3; Y(f32); Flags2(u32); FacialHair(u8); G7; Gender(u8);
Name(exact length, UTF-8, no NUL); Face(u8); G0; G2; H1; H7; X(f32);
Skin(u8); Race(u8); Level(u8); G6; H4; H0; G5; G1; Zone(i32).
Finally EACH restriction: Mask(i32), Race(u8). No orientation or guild name.
Flags and Flags2 are retained raw; FirstLogin is separate from customization.
Empty successful roster: 00 00 01 00 00 00 (no restrictions).

Conflicts: inherited WotLK world.go uses 0x0037/0x003B, a byte count, full GUID
and C-string name with different field ordering; none is reused. Clientless
agrees on mask/byte order but interprets four bags as display/enchant/type,
whereas TCPP writes ALL 23 entries as type/display/enchant. Follow TCPP.
Clientless discards restrictions/Success and treats zero characters as an
error; this probe parses restrictions, requires Success and accepts zero.
Clientless automatically logs in after selection; this probe never does.

TCPP compresses bodies larger than 0x400 using a persistent zlib stream and
Z_SYNC_FLUSH; opcode gains 0x8000 and payload begins with LE32 original size.
Compression state must include earlier compressed unsolicited packets.
The bounded one-shot probe replays accumulated compressed bytes to validate
exact cumulative decompressed lengths without retaining background readers.
This is deliberately bounded to 4 MiB total and 1 MiB per packet.

Server handler may perform its usual expired-ban cleanup and appearance
validation internally. No direct SQL, fixtures, character creation, server
changes or protected-runtime changes are part of this milestone.