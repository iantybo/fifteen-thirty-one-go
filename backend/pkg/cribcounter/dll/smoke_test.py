import ctypes, json, sys

dll = ctypes.CDLL(sys.argv[1])
dll.CribCountHand.restype = ctypes.c_void_p
dll.CribCountHand.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
dll.CribCountPegging.restype = ctypes.c_void_p
dll.CribCountPegging.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
dll.CribFree.argtypes = [ctypes.c_void_p]

def call(fn, *args):
    ptr = fn(*args)
    s = ctypes.cast(ptr, ctypes.c_char_p).value.decode()
    dll.CribFree(ptr)
    return json.loads(s)

hand = call(dll.CribCountHand, b"5H 5S 5D JC", b"5C", 0)
print("perfect-29 hand ->", hand)
assert hand["total"] == 29, hand

crib = call(dll.CribCountHand, b"2H 4H 6H 9H", b"10S", 1)
print("crib (no 4-flush) ->", crib)
assert crib["flush"] == 0 and crib["total"] == 4, crib

peg = call(dll.CribCountPegging, b"7H", b"8S", 7)
print("pegging 15 ->", peg)
assert peg["points"] == 2 and peg["new_total"] == 15, peg

bad = call(dll.CribCountHand, b"5H 5S", b"5C", 0)
print("error case ->", bad)
assert "error" in bad, bad

print("ALL DLL SMOKE TESTS PASSED")
