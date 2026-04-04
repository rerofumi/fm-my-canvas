<script lang="ts">
  import logo from './assets/images/logo-universal.png';
  
  let name = $state('');
  let result = $state('');

  async function greet() {
    if (name === '') return;
    try {
      const { Greet } = await import('../wailsjs/go/main/App');
      result = await Greet(name);
    } catch (err) {
      console.error(err);
    }
  }
</script>

<main>
  <img src={logo} alt="Wails Logo" class="logo" />
  
  <div class="result">{result || 'Please enter your name below 👇'}</div>
  
  <div class="input-box">
    <input 
      type="text" 
      bind:value={name} 
      placeholder="Enter your name"
      onkeydown={(e) => e.key === 'Enter' && greet()}
    />
    <button onclick={greet}>Greet</button>
  </div>
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
    gap: 1rem;
  }

  .logo {
    width: 200px;
    height: auto;
  }

  .result {
    font-size: 1.2rem;
    min-height: 1.5em;
  }

  .input-box {
    display: flex;
    gap: 0.5rem;
  }

  input {
    padding: 0.5rem 1rem;
    font-size: 1rem;
    border: 1px solid #ccc;
    border-radius: 4px;
  }

  button {
    padding: 0.5rem 1.5rem;
    font-size: 1rem;
    background-color: #007bff;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }

  button:hover {
    background-color: #0056b3;
  }
</style>
