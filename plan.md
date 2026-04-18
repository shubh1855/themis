# Single-File Python Chess Game Plan

## Project Summary
A fully functional, single-file chess game implemented in Python. The application features a graphical user interface (GUI) for piece movement and basic move validation logic.

## Stack and Rationale
- **Language:** Python 3.x
- **GUI Library:** `tkinter` (Standard Library)
  - *Rationale:* Zero external dependencies, ensuring the single-file distribution is portable and runs immediately on any standard Python installation.

## Folder Structure
Since this is a single-file project, the structure is as follows:
- `chess_game.py` (Contains all logic, classes, and GUI code)

## Class Architecture

### 1. `Piece` (Base Class)
- **Attributes:** `color` (White/Black), `name` (Pawn, Rook, etc.), `symbol` (Unicode character for display).
- **Methods:** `is_valid_move(start_pos, end_pos, board)` $\rightarrow$ returns Boolean.

### 2. `Piece Subclasses` (Pawn, Rook, Knight, Bishop, Queen, King)
- Each implements its own `is_valid_move` logic based on chess rules.

### 3. `Board` (Game Logic)
- **Attributes:** `grid` (8x8 2D list containing Piece objects or None), `turn` (White/Black).
- **Methods:** 
  - `move_piece(start_pos, end_pos)`: Validates move via the Piece class and updates grid.
  - `get_piece(pos)`: Returns piece at coordinate.
  - `switch_turn()`: Toggles active player.

### 4. `ChessGUI` (Presentation Layer)
- **Attributes:** `root` (Tk instance), `canvas` (Tk Canvas), `selected_square` (tracking current click).
- **Methods:**
  - `draw_board()`: Renders the 8x8 grid.
  - `draw_pieces()`: Places Unicode chess symbols on the canvas.
  - `handle_click(event)`: Manages the selection and movement flow.

## Dependencies
- None (Standard Library only).

## Risk Register
| Risk | Likelihood | Mitigation |
| :--- | :--- | :--- |
| Complex move validation (Castling/En Passant) | High | Implement basic piece moves first; mark advanced moves as deferred. |
| GUI Layout issues on different OS | Low | Use fixed square sizes (e.g., 60x60) for consistent rendering. |
| Single-file bloat | Low | Use clean class separations to keep the file readable. |

## Milestones

### M1: Core Logic & Piece Definitions
- **Owner:** Hephaestus
- **DoD:** `Board` and `Piece` classes implemented; unit-testable move validation for all pieces (excluding special moves like castling).

### M2: GUI Implementation
- **Owner:** Hephaestus
- **DoD:** `tkinter` window renders a board and places pieces using Unicode symbols; pieces can be moved via clicking.

### M3: Integration & Polish
- **Owner:** Hephaestus
- **DoD:** Turn-based logic enforced; invalid moves trigger a visual warning or are ignored; final single-file cleanup.

### M4: Verification
- **Owner:** Ares
- **DoD:** All pieces move according to basic rules; no crashes on invalid inputs; GUI is responsive.

## Deferred Scope
- Advanced chess rules: Castling, En Passant, Pawn Promotion.
- Check/Checkmate detection (Basic move validation only).
- AI opponent (Human vs Human only).
- Save/Load game state.
