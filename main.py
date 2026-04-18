from src.models import Pokemon, Move
from src.battle import Battle

def main():
    # Define some moves
    tackle = Move(name="Tackle", power=40, type="Normal")
    ember = Move(name="Ember", power=60, type="Fire")
    water_gun = Move(name="Water Gun", power=60, type="Water")
    vine_whip = Move(name="Vine Whip", power=60, type="Grass")

    # Define Pokemon
    charmander = Pokemon(name="Charmander", hp=100, attack=52, defense=43, moves=[tackle, ember])
    squirtle = Pokemon(name="Squirtle", hp=100, attack=48, defense=65, moves=[tackle, water_gun])

    print(f"A wild battle begins! {charmander} vs {squirtle}")
    
    battle = Battle(charmander, squirtle)
    
    # Simulating a battle with hardcoded move selections (index 0 or 1)
    winner = None
    while not winner:
        # For simulation, we'll just pick move index 0 or 1 based on turn
        p1_move = 1 if battle.turn % 2 == 0 else 0
        p2_move = 0 if battle.turn % 2 == 0 else 1
        
        winner = battle.execute_turn(p1_move, p2_move)
        print(f"Current status: {charmander} | {squirtle}\n")

    if winner == "P1":
        print(f"Winner: {charmander.name}!")
    else:
        print(f"Winner: {squirtle.name}!")

if __name__ == "__main__":
    main()