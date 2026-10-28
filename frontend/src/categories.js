const CATEGORIES = {
  PMD: { label: 'PMD', description: 'Plastic, metal, drink cartons', color: 'bg-blue-100 text-blue-800', bar: 'bg-blue-500' },
  ORGANIC: { label: 'Organic', description: 'Food waste, plants', color: 'bg-green-100 text-green-800', bar: 'bg-green-500' },
  PAPER: { label: 'Paper', description: 'Paper, cardboard', color: 'bg-amber-100 text-amber-800', bar: 'bg-amber-500' },
  GENERAL_WASTE: { label: 'General waste', description: 'Non-recyclable waste', color: 'bg-gray-200 text-gray-800', bar: 'bg-gray-500' },
  UNSUPPORTED: { label: 'Not supported', description: 'This bin cannot sort it', color: 'bg-red-100 text-red-800', bar: 'bg-red-500' },
}

export function categoryInfo(category) {
  return CATEGORIES[category] ?? { label: category || 'Unknown', description: '', color: 'bg-gray-100 text-gray-700', bar: 'bg-gray-400' }
}
