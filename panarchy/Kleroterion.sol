// An opt-in random number generator that uses potentially every single person in BitPeople (thus potentially every person in the world) to generate a random number.
// It uses a commit-reveal scheme, combined with "mutating" the committed value (thus, no one is able to know what they vote for, but they can ensure their vote is random).
// The RNG selects a number between 0 and N, where N is the number of people who participate. The result conforms to Poisson distribution e^-1/k!. The RNG is intended to be 
// used by BitPeople, to shuffle the registered population using Fisher-Yates shuffle, and, it can be used by any other application on the ledger as well.

contract Kleroterion is Mixer {

    BitPeople bitpeople = BitPeople(0x0000000000000000000000000000000000000005);

    mapping (uint => mapping (address => bytes32)) commit;
    mapping (uint => uint) votes;
    mapping (uint => mapping (uint => uint)) points;
    mapping (uint => uint) highscore;
    mapping (uint => uint) public winner;
    mapping (uint => mapping (address => bool)) claimedRandomToken;

    function commitHash(bytes32 _hash) public {
        uint t = schedule.schedule();
        require(schedule.quarter(t) < 2 && balanceOf[t][msg.sender] >= 1);
        balanceOf[t][msg.sender]--;
        commit[t][msg.sender] = _hash;
        votes[t]++;
    }
    function revealHash(bytes32 _preimage) public {
        uint t = schedule.schedule();
        require(schedule.quarter(t) == 2 && keccak256(abi.encode(_preimage)) == commit[t-1][msg.sender]);
        bytes32 mutated = keccak256(abi.encode(_preimage, winner[_t-1]));
        uint id = uint(mutated)%votes[t];
        points[t][id]++;
        if (points[t][id] > highscore[t]) {
            highscore[t]++;
            winner[t] = id;
        }
        delete commit[t-1][msg.sender];
    }
    function allocateRandomToken() public {
        uint t = schedule.schedule();
        require(bitpeople.proofOfUniqueHuman(t, msg.sender) && !claimedRandomToken[t][msg.sender]);
        balanceOf[t][msg.sender]++;
        claimedRandomToken[t][msg.sender] = true;
    }
}
